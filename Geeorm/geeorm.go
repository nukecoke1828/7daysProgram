package geeorm

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/nukecoke1828/7daysProgram/Geeorm/dialect"
	"github.com/nukecoke1828/7daysProgram/Geeorm/log"
	"github.com/nukecoke1828/7daysProgram/Geeorm/session"
)

type TxFunc func(*session.Session) (interface{}, error) // 事务处理函数类型

type Engine struct { // 与用户交互的接口
	db      *sql.DB
	dialect dialect.Dialect
}

func NewEngine(driver, source string) (e *Engine, err error) {
	db, err := sql.Open(driver, source) // 解析DSN、注册驱动、初始化连接池对象(没有真正连接数据库)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if err = db.Ping(); err != nil { // 测试数据库连接是否正常
		log.Error(err)
		return nil, err
	}
	dial, ok := dialect.GetDialect(driver) // 根据驱动名获取对应的方言对象
	if !ok {
		log.Errorf("dialect %s not found", driver)
		return
	}
	e = &Engine{db: db, dialect: dial}
	log.Info("Connect database success")
	return
}

func (engine *Engine) Close() {
	if err := engine.db.Close(); err != nil {
		log.Error("Failed to close database")
	}
	log.Info("Close database success")
}

func (engine *Engine) NewSession() *session.Session {
	return session.New(engine.db, engine.dialect)
}

// Transaction 事务处理
func (engine *Engine) Transaction(f TxFunc) (result interface{}, err error) {
	s := engine.NewSession()
	if err := s.Begin(); err != nil {
		return nil, err
	}
	defer func() {
		// recover() 只在 defer 函数内部 且 当前 goroutine 正在 panic 时才返回非 nil
		if p := recover(); p != nil { // 当f(s)发生panic时，捕获panic，回滚事务并重新抛出panic
			_ = s.Rollback()
			panic(p) // 重新抛出panic, 让上层调用者感知panic
		} else if err != nil { // 当f(s)发生错误时，回滚事务
			_ = s.Rollback()
		} else { // 当f(s)正常返回时，提交事务
			err = s.Commit() // 提交成功err为nil，否则为非nil
		}
	}()
	return f(s)
}

// difference 计算两个字符串数组的差集
func difference(a []string, b []string) (diff []string) {
	mapB := make(map[string]bool)
	for _, v := range b {
		mapB[v] = true
	}
	for _, v := range a {
		if _, ok := mapB[v]; !ok {
			diff = append(diff, v)
		}
	}
	return
}

// Migrate 迁移表结构
func (engine *Engine) Migrate(value interface{}) error {
	_, err := engine.Transaction(func(s *session.Session) (result interface{}, err error) {
		if !s.Model(value).HasTable() { // 表不存在
			log.Infof("table %s doesn't exist", s.RefTable().Name)
			return nil, s.CreateTable()
		}
		table := s.RefTable() // 结构体
		rows, _ := s.Raw(fmt.Sprintf("SELECT * FROM %s LIMIT 1", table.Name)).QueryRows()
		columns, _ := rows.Columns()                     // 字段名列表(数据库字段名)
		addCols := difference(table.FieldNames, columns) // 新增字段
		delCols := difference(columns, table.FieldNames) // 删除字段
		log.Infof("added cols %v, deleted cols %v", addCols, delCols)
		for _, col := range addCols {
			f := table.GetField(col)
			sqlStr := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table.Name, f.Name, f.Type) // 增加字段
			if _, err = s.Raw(sqlStr).Exec(); err != nil {
				return
			}
		}
		if len(delCols) == 0 { // 没有删除字段
			return
		}
		tmp := "tmp_" + table.Name
		fieldStr := strings.Join(table.FieldNames, ", ")                                       // 字段名列表(结构体字段名)
		s.Raw(fmt.Sprintf("CREATE TABLE %s AS SELECT %s FROM %s;", tmp, fieldStr, table.Name)) // 临时表
		s.Raw(fmt.Sprintf("DROP TABLE %s;", table.Name))                                       // 删除原表
		s.Raw(fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", tmp, table.Name))                    // 重命名临时表为原表
		_, err = s.Exec()                                                                      // 执行SQL语句
		return
	})
	return err
}
