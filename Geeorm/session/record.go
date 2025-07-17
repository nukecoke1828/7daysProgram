package session

import (
	"errors"
	"reflect"

	"github.com/nukecoke1828/7daysProgram/Geeorm/clause"
)

func (s *Session) Insert(values ...interface{}) (int64, error) {
	recordValues := make([]interface{}, 0) // 记录SQL语句中需要的值
	for _, value := range values {
		s.CallMethod(BeforeInsert, value)
		table := s.Model(value).RefTable() // 映射表结构
		// 设置 INSERT INTO tableName (col1,col2,...)
		s.clause.Set(clause.INSERT, table.Name, table.FieldNames)
		// 把单条结构体字段值摊平成切片，拼到 recordValues 末尾
		recordValues = append(recordValues, table.RecordValues(value))
	}
	s.clause.Set(clause.VALUES, recordValues...)              // 设置 VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?)
	sql, vars := s.clause.Build(clause.INSERT, clause.VALUES) // INSERT INTO User (Name,Age) VALUES (?,?), (?,?), (?,?)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterInsert, nil)
	return result.RowsAffected() // 返回受影响的行数
}

// 把整张表扫描进切片
// 接收指向切片的指针
func (s *Session) Find(values interface{}) error {
	s.CallMethod(BeforeQuery, nil)
	destSlice := reflect.Indirect(reflect.ValueOf(values))                // 得到切片的反射对象
	destType := destSlice.Type().Elem()                                   // 得到切片元素的类型
	table := s.Model(reflect.New(destType).Elem().Interface()).RefTable() // 映射表结构
	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	rows, err := s.Raw(sql, vars...).QueryRows() // 多行数据集合
	if err != nil {
		return err
	}
	for rows.Next() { // 循环读取每一行
		dest := reflect.New(destType).Elem() // 实例化元素,用于存放每一行数据
		var values []interface{}             // 临时切片，用来存放 每个字段的指针，供 rows.Scan 写入数据
		for _, name := range table.FieldNames {
			// 为每个字段取地址，放进 values，供 rows.Scan 写入
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		// 将数据写入对应地址, 即 dest 的字段
		if err := rows.Scan(values...); err != nil {
			return err
		}
		s.CallMethod(AfterQuery, dest.Addr().Interface())
		destSlice.Set(reflect.Append(destSlice, dest)) // 追加到切片末尾
	}
	return rows.Close()
}

// 支持 map[string]interface{} 形式和kv list: "Name", "Tom", "Age", 18, .... 形式的更新
func (s *Session) Update(kv ...interface{}) (int64, error) {
	s.CallMethod(BeforeUpdate, nil)
	m, ok := kv[0].(map[string]interface{}) // 类型断言
	if !ok {                                // 不是 map[string]interface{} 形式
		m = make(map[string]interface{})
		for i := 0; i < len(kv); i += 2 {
			// kv[i]   断言为 string → 列名
			// kv[i+1] 作为值
			m[kv[i].(string)] = kv[i+1]
		}
	}
	s.clause.Set(clause.UPDATE, s.RefTable().Name, m)
	sql, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterUpdate, nil)
	return result.RowsAffected() // 返回受影响的行数
}

// 根据条件删除数据
func (s *Session) Delete() (int64, error) {
	s.CallMethod(BeforeDelete, nil)
	s.clause.Set(clause.DELETE, s.RefTable().Name)
	sql, vars := s.clause.Build(clause.DELETE, clause.WHERE)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterDelete, nil)
	return result.RowsAffected() // 返回受影响的行数
}

// 根据条件查询总数
func (s *Session) Count() (int64, error) {
	s.clause.Set(clause.COUNT, s.RefTable().Name)
	sql, vars := s.clause.Build(clause.COUNT, clause.WHERE)
	row := s.Raw(sql, vars...).QueryRow() // 只返回一行数据
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// 限制返回的行数
func (s *Session) Limit(num int) *Session {
	s.clause.Set(clause.LIMIT, num)
	return s
}

// 限制条件
func (s *Session) Where(desc string, args ...interface{}) *Session {
	var vars []interface{}
	// append(vars, desc)
	// 把 desc 当成单个元素追加到 vars 末尾，得到：
	// [limitVal, "Name = ? AND Age > ?"]
	// 再 append(..., args...)
	// 把 args 的所有元素继续追加，得到：
	// [limitVal, "Name = ? AND Age > ?", "Tom", 18]
	// 最后的 ...
	// 把上一步得到的整体切片展开成可变参数，传给 clause.Set
	s.clause.Set(clause.WHERE, append(append(vars, desc), args...)...)
	return s
}

// 排序
func (s *Session) OrderBy(desc string) *Session {
	s.clause.Set(clause.ORDERBY, desc)
	return s
}

func (s *Session) First(value interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(value))              // 得到反射对象
	destSlice := reflect.New(reflect.SliceOf(dest.Type())).Elem() // 创建一个临时的反射对象类型的切片存储结果
	// destSlice.Addr()得到切片的地址，在调用Find()追加数据
	if err := s.Limit(1).Find(destSlice.Addr().Interface()); err != nil {
		return err
	}
	if destSlice.Len() == 0 {
		return errors.New("NOT FOUND")
	}
	dest.Set(destSlice.Index(0)) // 将第一个元素赋值给 value
	return nil
}
