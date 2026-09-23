// Package store is a librarry that uses ORM to persist the state of task nodes
package store

import (
	"context"
	"errors"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	db      *gorm.DB
	changes chan struct{}
}

type taskRow struct {
	ID                 int64  `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID           int64  `gorm:"column:parent_id"`
	Title              string `gorm:"column:title;not null"`
	Body               string `gorm:"column:body;not null"`
	ScheduleType       string `gorm:"column:schedule_type;not null"`
	ScheduleOffset     string `gorm:"column:schedule_offset"`
	ScheduleExpression string `gorm:"column:schedule_expression;not null"`
	ActionType         string `gorm:"column:action_type;not null"`
	ActionExpression   string `gorm:"column:action_expression;not null"`
	LastFired          string `gorm:"column:last_fired"`
	Enabled            bool   `gorm:"column:enabled"`
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
	if err := db.AutoMigrate(&taskRow{}); err != nil {
		return nil, err
	}
	store := Store{
		db: db,
	}
	return &store, err
}

func (store *Store) Changes() <-chan struct{} {
	return store.changes
}

func (store *Store) Close() error {
	sqlDB, err := store.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (store *Store) Add(ctx context.Context, task taskRow) (int64, error) {
	if err := store.db.WithContext(ctx).Create(&task).Error; err != nil {
		return -1, err
	}
	return task.ID, nil
}

func (store *Store) Remove(ctx context.Context, id int64) error {
	res := store.db.WithContext(ctx).Where("id = ?", id).Delete(&taskRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("task not found")
	}
	return nil
}

func (store *Store) Update(ctx context.Context, task taskRow) error {
	res := store.db.WithContext(ctx).Model(&taskRow{}).Where("id = ?", task.ID).
		Select("title", "body", "schedule_type", "schedule_expression",
			"action_type", "action_config", "enabled").Updates(task)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("task not found")
	}
	return nil
}

func (store *Store) Get(ctx context.Context, id int64) (taskRow, error) {
	var r taskRow
	if err := store.db.WithContext(ctx).First(&r, id).Error; err != nil {
		return taskRow{}, err
	}
	return r, nil
}

func (store *Store) GetAll(ctx context.Context) ([]taskRow, error) {
	var rows []taskRow
	if err := store.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
