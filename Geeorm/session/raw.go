package session

import (
	"database/sql"
	"strings"

	"github.com/nukecoke1828/7daysProgram/Geeorm/clause"
	"github.com/nukecoke1828/7daysProgram/Geeorm/dialect"
	"github.com/nukecoke1828/7daysProgram/Geeorm/log"
	"github.com/nukecoke1828/7daysProgram/Geeorm/schema"
)

var _ CommonDB = (*sql.DB)(nil) // 确保 CommonDB 接口实现
var _ CommonDB = (*sql.Tx)(nil) // 确保 CommonDB 接口实现

type Session struct { // 与数据库交互的会话
	db       *sql.DB         // 数据库连接池
	sql      strings.Builder // sql缓冲区
	sqlVars  []interface{}   // sql参数列表
	dialect  dialect.Dialect // 数据库方言
	refTable *schema.Schema  // 引用的表结构
	clause   clause.Clause   // SQL子句组合
	tx       *sql.Tx         // 事务
}

// CommonDB 通用数据库接口，包含sql.DB和sql.Tx的接口方法
type CommonDB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// New 创建一个新的会话
func New(db *sql.DB, dialect dialect.Dialect) *Session {
	return &Session{
		db:      db,
		dialect: dialect,
	}
}

func (s *Session) Clear() {
	s.sql.Reset()              // 清空sql缓冲区
	s.sqlVars = nil            // 清空sql参数列表
	s.clause = clause.Clause{} // 清空SQL子句组合
}

// DB 如果有事务，则返回事务对象，否则返回数据库连接池对象
func (s *Session) DB() CommonDB {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// 构建sql语句
func (s *Session) Raw(sql string, values ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ")
	s.sqlVars = append(s.sqlVars, values...)
	return s
}

func (s *Session) Exec() (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	// s.DB()	取底层 *sql.DB 连接池
	// s.sql.String()	把 strings.Builder 里的字节数组转成一个最终 SQL 字符串
	// s.sqlVars...	把切片里的参数 逐一展开
	if result, err = s.DB().Exec(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return result, err
}

// QueryRow 查询单条数据
func (s *Session) QueryRow() *sql.Row {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	return s.DB().QueryRow(s.sql.String(), s.sqlVars...)
}

// QueryRows 查询多条数据
func (s *Session) QueryRows() (rows *sql.Rows, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if rows, err = s.DB().Query(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return rows, err
}
