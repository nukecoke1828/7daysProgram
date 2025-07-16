package clause

import (
	"reflect"
	"testing"
)

func testSelect(t *testing.T) {
	var clause Clause
	clause.Set(LIMIT, 3)                      // LIMIT 3
	clause.Set(SELECT, "User", []string{"*"}) // SELECT * FROM User
	clause.Set(WHERE, "Name = ?", "Tom")      // WHERE Name = ?
	clause.Set(ORDERBY, "Age ASC")            // ORDER BY Age ASC
	sql, vars := clause.Build(SELECT, WHERE, ORDERBY, LIMIT)
	t.Log(sql, vars) // 打印SQL语句及参数
	if sql != "SELECT * FROM User WHERE Name = ? ORDER BY Age ASC LIMIT ?" {
		t.Fatal("failed to build SQL")
	}
	if !reflect.DeepEqual(vars, []interface{}{"Tom", 3}) { // 确保参数顺序和值完全匹配
		t.Fatal("failed to build SQLVars")
	}
}

func TestClause_Build(t *testing.T) {
	t.Run("select", func(t *testing.T) { // 启动子测试select
		testSelect(t)
	})
}
