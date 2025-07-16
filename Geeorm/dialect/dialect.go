package dialect

import (
	"reflect"
)

var dialectsMap = map[string]Dialect{}

type Dialect interface {
	DataTypeOF(typ reflect.Value) string                    // 将go语言类型转换成数据库字段类型
	TableExistSQL(tableName string) (string, []interface{}) // 检查表是否存在的SQL语句及参数
}

// RegisterDialect 注册一个数据库方言
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

// GetDialect 获取一个数据库方言
func GetDialect(name string) (dialect Dialect, ok bool) {
	dialect, ok = dialectsMap[name]
	return
}
