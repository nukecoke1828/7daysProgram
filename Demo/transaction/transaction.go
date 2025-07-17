package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3" // 匿名导入自动注册sqlite3驱动
)

func main() {
	db, _ := sql.Open("sqlite3", "gee.db")
	defer func() { _ = db.Close() }()
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS User(`Name` text);")

	tx, _ := db.Begin() // 开启事务
	_, err1 := tx.Exec("INSERT INTO User(`Name`) VALUES (?)", "Tom")
	_, err2 := tx.Exec("INSERT INTO User(`Name`) VALUES (?)", "Jack")
	if err1 != nil || err2 != nil {
		_ = tx.Rollback() // 回滚事务
		log.Println("Rollback", err1, err2)
	} else {
		_ = tx.Commit() // 提交事务
		log.Println("Commit")
	}
}
