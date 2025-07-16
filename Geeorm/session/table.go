package session

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/nukecoke1828/7daysProgram/Geeorm/log"
	"github.com/nukecoke1828/7daysProgram/Geeorm/schema"
)

// 延迟解析结构体
func (s *Session) Model(value interface{}) *Session {
	// 解析表结构(只有在第一次调用或换结构体时才真正解析)
	if s.refTable == nil || reflect.TypeOf(value) != reflect.TypeOf(s.refTable.Model) {
		s.refTable = schema.Parse(value, s.dialect)
	}
	return s
}

// 获取引用的表结构
func (s *Session) RefTable() *schema.Schema {
	if s.refTable == nil {
		log.Error("Model is not set")
	}
	return s.refTable
}

func (s *Session) CreateTable() error {
	table := s.RefTable()
	var columns []string                 // 字段列表(字段名 字段类型 标签)
	for _, field := range table.Fields { // 将表中的字段信息转换为SQL语句中的字段列表
		columns = append(columns, fmt.Sprintf("%s %s %s", field.Name, field.Type, field.Tag))
	}
	desc := strings.Join(columns, ", ") // 将字段列表用逗号分隔
	_, err := s.Raw(fmt.Sprintf("CREATE TABLE %s (%s);", table.Name, desc)).Exec()
	return err
}

func (s *Session) DropTable() error {
	_, err := s.Raw(fmt.Sprintf("DROP TABLE IF EXISTS %s;", s.RefTable().Name)).Exec()
	return err
}

func (s *Session) HasTable() bool {
	sql, values := s.dialect.TableExistSQL(s.RefTable().Name) // 获取检查表是否存在的SQL语句及参数
	row := s.Raw(sql, values...).QueryRow()
	var tmp string
	_ = row.Scan(&tmp)
	return tmp == s.RefTable().Name
}
