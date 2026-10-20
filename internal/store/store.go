// Package store persists tasks and their execution state using GORM.
package store

import (
	"context"
	"errors"
	"reflect"
	"time"

	"butler/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	db          *gorm.DB
	typeChannel chan reflect.Type
}

func NewStore(dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"
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
	if err := db.AutoMigrate(&model.Task{}, &model.Config{}, &model.User{}); err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	store := Store{
		db:          db,
		typeChannel: make(chan reflect.Type, 1),
	}
	return &store, nil
}

func reportChange[T any](store *Store) {
	select {
	case store.typeChannel <- reflect.TypeFor[T]():
	default:
	}
}

func (store *Store) Changes() <-chan reflect.Type {
	return store.typeChannel
}

func (store *Store) Close() error {
	sqlDB, err := store.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func Add[T any](ctx context.Context, store *Store, value *T) error {
	if err := gorm.G[T](store.db).Create(ctx, value); err != nil {
		return err
	}
	reportChange[T](store)
	return nil
}

func Remove[T any](ctx context.Context, store *Store, id int64) error {
	affected, err := gorm.G[T](store.db).
		Where("id = ?", id).
		Delete(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return gorm.ErrRecordNotFound
	}
	reportChange[T](store)
	return nil
}

func (store *Store) MarkLastFired(ctx context.Context, id int64, at time.Time) error {
	return store.db.WithContext(ctx).Model(&model.Task{}).
		Where("id = ?", id).Update("last_fired", at.UTC()).Error
}

func (store *Store) Update(ctx context.Context, task model.Task) error {
	if len(task.ActionExpression) == 0 {
		task.ActionExpression = []byte("{}")
	}
	res := store.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", task.ID).
		Select("title", "body", "schedule_type", "schedule_offset", "schedule_expression",
			"action_type", "action_expression", "channel", "enabled").Updates(task)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("task not found")
	}
	reportChange[model.Task](store)
	return nil
}

func Get[T any](ctx context.Context, store *Store, id int64) (T, error) {
	return gorm.G[T](store.db).Where("id=?", id).First(ctx)
}

func GetAll[T any](ctx context.Context, store *Store) ([]T, error) {
	return gorm.G[T](store.db).Find(ctx)
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
