package clause

import (
	"fmt"
	"strings"
)

type generator func(values ...interface{}) (string, []interface{}) // SQL子句生成器函数类型

var generators map[Type]generator // SQL子句生成器映射表

// 初始化SQL子句生成器映射表
func init() {
	generators = make(map[Type]generator)
	generators[INSERT] = _insert
	generators[VALUES] = _values
	generators[SELECT] = _select
	generators[LIMIT] = _limit
	generators[WHERE] = _where
	generators[ORDERBY] = _orderby
	generators[UPDATE] = _update
	generators[DELETE] = _delete
	generators[COUNT] = _count
}

// 生成占位符,防止SQL注入
func genBindVars(num int) string {
	var vars []string
	for i := 0; i < num; i++ {
		vars = append(vars, "?")
	}
	return strings.Join(vars, ", ")
}

// 输入
// 1.表名
// 2.字段列表
func _insert(values ...interface{}) (string, []interface{}) {
	tableName := values[0]                             // 表名
	fields := strings.Join(values[1].([]string), ", ") // 字段名
	return fmt.Sprintf("INSERT INTO %s (%v)", tableName, fields), []interface{}{}
}

func _values(values ...interface{}) (string, []interface{}) {
	var bindStr string             // 占位符模版字符串(决定了参数的格式)
	var sql strings.Builder        // SQL语句构建器
	var vars []interface{}         // 占位符参数列表
	sql.WriteString("VALUES ")     // 写入固定前缀
	for i, value := range values { // 遍历每行数据
		v := value.([]interface{}) // 类型断言获取值切片
		if bindStr == "" {         // 第一次循环，生成占位符模版字符串
			bindStr = genBindVars(len(v)) // 生成占位符
		}
		sql.WriteString(fmt.Sprintf("(%v)", bindStr)) // 添加带括号的值组
		if i+1 != len(values) {
			sql.WriteString(", ")
		}
		vars = append(vars, v...) // 添加参数列表
	}
	return sql.String(), vars
}

// 查找语句
func _select(values ...interface{}) (string, []interface{}) {
	tableName := values[0]
	fields := strings.Join(values[1].([]string), ", ")
	return fmt.Sprintf("SELECT %v FROM %s", fields, tableName), []interface{}{}
}

// 限制查询结果数量
func _limit(values ...interface{}) (string, []interface{}) {
	return "LIMIT ?", values
}

// 输入
// 1.条件描述
// 2.参数列表
func _where(values ...interface{}) (string, []interface{}) {
	desc, vars := values[0], values[1:]
	return fmt.Sprintf("WHERE %s", desc), vars
}

// 根据字段排序
func _orderby(values ...interface{}) (string, []interface{}) {
	return fmt.Sprintf("ORDER BY %s", values[0]), []interface{}{}
}

func _update(values ...interface{}) (string, []interface{}) {
	tableName := values[0]
	m := values[1].(map[string]interface{}) // 类型断言获取值映射表
	var keys []string                       // 键列表
	var vars []interface{}                  // 值列表
	for k, v := range m {
		keys = append(keys, k+"= ?")
		vars = append(vars, v)
	}
	return fmt.Sprintf("UPDATE %s SET %s", tableName, strings.Join(keys, ", ")), vars
}

func _delete(values ...interface{}) (string, []interface{}) {
	return fmt.Sprintf("DELETE FROM %s", values[0]), []interface{}{}
}

// 统计满足条件的记录数量
func _count(values ...interface{}) (string, []interface{}) {
	// 替换为SELECT COUNT(*) FROM tableName
	return _select(values[0], []string{"COUNT(*)"})
}
