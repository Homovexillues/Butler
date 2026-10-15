// Package store persists tasks and their execution state using GORM.
package store

import (
	"context"
	"errors"
	"time"

	"butler/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	db           *gorm.DB
	nodesChannel chan struct{}
}

func NewStore(dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?_journal_mode=WAL&_fk=1&_txlock=immediate&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxIdleTime(0)
	if err := db.AutoMigrate(&model.Task{}); err != nil {
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
	sqlDB, err := store.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (store *Store) Add(ctx context.Context, task model.Task) (int64, error) {
	taskExist, err := store.Exist(ctx, task)
	if err != nil {
		return -1, err
	}
	if taskExist {
		return -1, errors.New("task with the same title and body already exists")
	}
	if err := store.db.WithContext(ctx).Create(&task).Error; err != nil {
		return -1, err
	}
	store.reportChange()
	return task.ID, nil
}

func (store *Store) Remove(ctx context.Context, id int64) error {
	res := store.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Task{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("task not found")
	}
	store.reportChange()
	return nil
}

func (store *Store) MarkLastFired(ctx context.Context, id int64, at time.Time) error {
	return store.db.WithContext(ctx).Model(&model.Task{}).
		Where("id = ?", id).Update("last_fired", at).Error
}

func (store *Store) Update(ctx context.Context, task model.Task) error {
	res := store.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", task.ID).
		Select("title", "body", "schedule_type", "schedule_offset", "schedule_expression",
			"action_type", "action_expression", "channel", "enabled").Updates(task)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("task not found")
	}
	store.reportChange()
	return nil
}

func (store *Store) Get(ctx context.Context, id int64) (model.Task, error) {
	var r model.Task
	if err := store.db.WithContext(ctx).First(&r, id).Error; err != nil {
		return model.Task{}, err
	}
	return r, nil
}

func (store *Store) GetAll(ctx context.Context) ([]model.Task, error) {
	var rows []model.Task
	if err := store.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (store *Store) Exist(ctx context.Context, task model.Task) (bool, error) {
	var r model.Task
	err := store.db.WithContext(ctx).
		Where("title = ? AND body = ?", task.Title, task.Body).
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
