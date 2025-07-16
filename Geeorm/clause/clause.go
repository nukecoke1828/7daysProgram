package clause

import "strings"

type Type int // SQL子句类型

const ( // SQL子句类型
	INSERT Type = iota
	VALUES
	SELECT
	LIMIT
	WHERE
	ORDERBY
	UPDATE
	DELETE
	COUNT
)

type Clause struct { // SQL子句组合
	sql     map[Type]string        // 存储各个子句的SQL片段
	sqlVars map[Type][]interface{} // 存储各个子句的SQL参数
}

// 根据Type获取对应的SQL语句,并存入Clause中
func (c *Clause) Set(name Type, vars ...interface{}) {
	if c.sql == nil { // 初始化
		c.sql = make(map[Type]string)
		c.sqlVars = make(map[Type][]interface{})
	}
	sql, vars := generators[name](vars...)
	c.sql[name] = sql
	c.sqlVars[name] = vars
}

// 根据Type的顺序构造最终的SQL语句
func (c *Clause) Build(orders ...Type) (string, []interface{}) {
	var sqls []string              // 存储各个子句的SQL片段
	var vars []interface{}         // 存储各个子句的SQL参数
	for _, order := range orders { // 按指定顺序遍历子句类型
		if sql, ok := c.sql[order]; ok { // 检查是否存在该子句
			sqls = append(sqls, sql)
			vars = append(vars, c.sqlVars[order]...)
		}
	}
	return strings.Join(sqls, " "), vars // 合并SQL片段并返回
}
