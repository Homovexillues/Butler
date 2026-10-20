package model

type User struct {
	ID       uint   `gorm:"column:id;primaryKey"`
	Name     string `gorm:"column:name;uniqueIndex"`
	Password string `gorm:"column:password"`
}
