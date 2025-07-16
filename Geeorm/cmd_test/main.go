package main

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	geeorm "github.com/nukecoke1828/7daysProgram/Geeorm"
)

func main() {
	engine, _ := geeorm.NewEngine("sqlite3", "gee.db")
	defer engine.Close()
	s := engine.NewSession() // 获取一个会话
	_, _ = s.Raw("DROP TABLE IF EXISTS User;").Exec()
	_, _ = s.Raw("CREATE TABLE User(Name text);").Exec()
	_, _ = s.Raw("CREATE TABLE User(Name text);").Exec()
	result, _ := s.Raw("INSERT INTO User(`Name`) VALUES (?), (?)", "Tom", "Jerry").Exec()
	count, _ := result.RowsAffected()
	fmt.Printf("Exec success, %d affected\n", count)
}
