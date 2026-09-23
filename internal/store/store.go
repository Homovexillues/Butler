// Package store is a librarry that uses ORM to persist the state of task nodes
package store

import (
	"butler/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

type taskRow struct {
	ID                 int64  `gorm:"column:Id;primaryKey;autoIncrement"`
	Title              string `gorm:"column:Title;not null"`
	Body               string `gorm:"column:Body;not null"`
	ScheduleType       string `gorm:"column:ScheduleType;not null"`
	ScheduleOffset     string `gorm:"column:ScheduleOffset"`
	ScheduleExpression string `gorm:"column:ScheduleExpression;not null"`
	ActionType         string `gorm:"column:ActionType;not null"`
	ActionExpression   string `gorm:"column:ActionExpression;not null"`
	LastFired          string `gorm:"column:LastFired"`
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

func (store *Store) Close() error {
	sqlDB, err := store.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (store *Store) Add(node *model.Node) {
}
func (store *Store) Remove() {}
func (store *Store) Update() {}
func (store *Store) Find()   {}
