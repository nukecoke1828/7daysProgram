package geeorm

import (
	"database/sql"

	"github.com/nukecoke1828/7daysProgram/Geeorm/dialect"
	"github.com/nukecoke1828/7daysProgram/Geeorm/log"
	"github.com/nukecoke1828/7daysProgram/Geeorm/session"
)

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
