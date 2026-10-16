// Package store persists tasks and their execution state using sqlite.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"butler/internal/model"

	"modernc.org/sqlite"
)

type Store struct {
	db           *sql.DB
	nodesChannel chan struct{}
}

func NewStore(dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?_journal_mode=WAL&_fk=1&_txlock=immediate&_busy_timeout=5000"
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(0)
	if err := Init(db); err != nil {
		err = errors.Join(err, db.Close())
		return nil, err
	}
	store := Store{
		db:           db,
		nodesChannel: make(chan struct{}, 1),
	}
	return &store, nil
}

func (store *Store) reportChange() {
	select {
	case store.nodesChannel <- struct{}{}:
	default:
	}
}

func (store *Store) Changes() <-chan struct{} {
	return store.nodesChannel
}

func (store *Store) Close() error {
	return store.db.Close()
}

func Init(db *sql.DB) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			schedule_type TEXT NOT NULL,
			schedule_expression TEXT NOT NULL,
			schedule_offset TEXT,
			action_type TEXT NOT NULL,
			action_expression TEXT NOT NULL,
			channel TEXT NOT NULL,
			enabled BOOL NOT NULL,
			last_fired DATETIME
		);
		`)
	return err
}

func (store *Store) Add(ctx context.Context, task model.Task) (int64, error) {
	taskExist, err := store.Exist(ctx, task)
	if err != nil {
		return -1, err
	}
	if taskExist {
		return -1, errors.New("task with the same title and body already exists")
	}
	offsets, err := json.Marshal(task.ScheduleOffset)
	if err != nil {
		return -1, err
	}
	channels, err := json.Marshal(task.Channels)
	if err != nil {
		return -1, err
	}
	actionExpression := string(task.ActionExpression)
	if actionExpression == "" {
		actionExpression = "{}"
	}
	result, err := store.db.ExecContext(ctx, `
		INSERT INTO tasks (
			title,body,
			schedule_type,schedule_expression,schedule_offset,
			action_type,action_expression,
			channel,enabled,last_fired
		) VALUES (?,?,?,?,?,?,?,?,?,?)
		`,
		task.Title, task.Body,
		string(task.ScheduleType), task.ScheduleExpression, string(offsets),
		string(task.ActionType), actionExpression,
		channels, task.Enabled, encodeTime(task.LastFired),
	)
	if err != nil {
		return -1, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return -1, err
	}
	store.reportChange()
	return id, nil
}

func (store *Store) Remove(ctx context.Context, id int64) error {
	result, err := store.db.ExecContext(ctx, `
		DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("task not found")
	}
	store.reportChange()
	return nil
}

func (store *Store) MarkLastFired(ctx context.Context, id int64, at time.Time) error {
	_, err := store.db.ExecContext(ctx, `
			UPDATE tasks SET
				last_fired = ?
			WHERE id = ?
		`, encodeTime(&at), id)
	return err
}

func (store *Store) Update(ctx context.Context, task model.Task) error {
	offsets, err := json.Marshal(task.ScheduleOffset)
	if err != nil {
		return err
	}
	channels, err := json.Marshal(task.Channels)
	if err != nil {
		return err
	}
	actionExpression := string(task.ActionExpression)
	if actionExpression == "" {
		actionExpression = "{}"
	}
	result, err := store.db.ExecContext(ctx, `
		UPDATE tasks SET
			title = ?,body = ?,
			schedule_type = ?,schedule_expression = ?,schedule_offset = ?,
			action_type = ?,action_expression = ?,
			channel = ?,enabled = ?
		WHERE id = ?	
		`,
		task.Title, task.Body,
		string(task.ScheduleType), task.ScheduleExpression, string(offsets),
		string(task.ActionType), actionExpression,
		channels, task.Enabled, task.ID,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("task not found")
	}
	store.reportChange()
	return nil
}

func (store *Store) Get(ctx context.Context, id int64) (model.Task, error) {
	var task model.Task
	var offsetsJSON, channelsJSON, lastFired sql.NullString
	var actionJSON string

	if err := store.db.QueryRowContext(ctx, `
			SELECT 
				id,
				title,body,
				schedule_type,schedule_expression,schedule_offset,
				action_type,action_expression,
				channel,enabled,last_fired
 			FROM tasks WHERE id = ?
		`, id).Scan(
		&task.ID,
		&task.Title, &task.Body,
		&task.ScheduleType, &task.ScheduleExpression, &offsetsJSON,
		&task.ActionType, &actionJSON,
		&channelsJSON, &task.Enabled, &lastFired,
	); err != nil {
		return model.Task{}, err
	}
	task.ActionExpression = json.RawMessage(actionJSON)
	if offsetsJSON.Valid {
		if err := json.Unmarshal([]byte(offsetsJSON.String), &task.ScheduleOffset); err != nil {
			return task, err
		}
	}
	if channelsJSON.Valid {
		if err := json.Unmarshal([]byte(channelsJSON.String), &task.Channels); err != nil {
			return task, err
		}
	}
	if lastFired.Valid {
		at, err := time.Parse(time.RFC3339Nano, lastFired.String)
		if err != nil {
			return model.Task{}, fmt.Errorf("task %d: invalid last_fired %q:%w", task.ID, lastFired.String, err)
		}
		task.LastFired = &at
	}
	return task, nil
}

func (store *Store) GetAll(ctx context.Context) (tasks []model.Task, err error) {
	tasks = make([]model.Task, 0)
	rows, err := store.db.QueryContext(ctx, `
		SELECT 
			id,
			title,body,
			schedule_type,schedule_expression,schedule_offset,
			action_type,action_expression,
			channel,enabled,last_fired
 		FROM tasks ORDER BY id
		`)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
		if err != nil {
			tasks = nil
		}
	}()
	for rows.Next() {
		var task model.Task
		var offsetsJSON, channelsJSON, lastFired sql.NullString
		var actionJSON string
		err := rows.Scan(
			&task.ID,
			&task.Title, &task.Body,
			&task.ScheduleType, &task.ScheduleExpression, &offsetsJSON,
			&task.ActionType, &actionJSON,
			&channelsJSON, &task.Enabled, &lastFired,
		)
		if err != nil {
			return nil, err
		}
		task.ActionExpression = json.RawMessage(actionJSON)
		if offsetsJSON.Valid {
			if err := json.Unmarshal([]byte(offsetsJSON.String), &task.ScheduleOffset); err != nil {
				return nil, err
			}
		}
		if channelsJSON.Valid {
			if err := json.Unmarshal([]byte(channelsJSON.String), &task.Channels); err != nil {
				return nil, err
			}
		}
		if lastFired.Valid {
			at, err := time.Parse(time.RFC3339Nano, lastFired.String)
			if err != nil {
				return nil, fmt.Errorf("task %d: invalid last_fired %q:%w", task.ID, lastFired.String, err)
			}
			task.LastFired = &at
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (store *Store) Exist(ctx context.Context, task model.Task) (bool, error) {
	var exists bool
	err := store.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tasks
			WHERE title = ? AND body = ?
		)
		`, task.Title, task.Body).Scan(&exists)
	return exists, err
}

func encodeTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}
