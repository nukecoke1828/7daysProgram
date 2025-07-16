package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, _ := sql.Open("sqlite3", "gee.db")
	defer func() {
		_ = db.Close()
	}()
	_, _ = db.Exec("DROP TABLE IF EXISTS User;") //删除表如果存在
	_, _ = db.Exec("CREATE TABLE User(Name text);")
	result, err := db.Exec("INSERT INTO User(`Name`) VALUES(?), (?)", "Alice", "Bob")
	if err == nil { //如果插入成功,打印影响的行数
		affected, _ := result.RowsAffected()
		log.Println(affected)
	}
	row := db.QueryRow("SELECT Name FROM User LIMIT 1") //查询第一行
	var name string
	if err := row.Scan(&name); err == nil {
		log.Println(name)
	}
}
