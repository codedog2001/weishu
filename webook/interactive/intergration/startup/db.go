package startup

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"xiaoweishu/webook/interactive/repository/dao"
)

//startup的初始化是用来做集成测试的，也就是TDD test driven development

func InitDB() *gorm.DB {
	db, err := gorm.Open(mysql.Open("root:root@tcp(localhost:13316)/webook"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	err = dao.InitTables(db)
	if err != nil {
		panic(err)
	}
	return db
}
