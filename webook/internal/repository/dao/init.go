package dao

import (
	"gorm.io/gorm"
)

// InitTable 用grom的自动建表功能来建表
func InitTable(db *gorm.DB) error {
	return db.AutoMigrate(&User{},
		&Article{},
		&PublishedArticle{})
}
