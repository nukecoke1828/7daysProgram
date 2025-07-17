package session

import "github.com/nukecoke1828/7daysProgram/Geeorm/log"

func (s *Session) Begin() (err error) {
	log.Info("Begin transaction")
	if s.tx, err = s.db.Begin(); err != nil {
		log.Error(err)
		return
	}
	return
}

func (s *Session) Commit() (err error) {
	log.Info("Commit transaction")
	if err = s.tx.Commit(); err != nil {
		log.Error(err)
	}
	return
}

// Rollback 回滚事务(撤诉所有修改、释放锁、连接归还连接池) 不会进行重试
func (s *Session) Rollback() (err error) {
	log.Info("Rollback transaction")
	if err = s.tx.Rollback(); err != nil {
		log.Error(err)
	}
	return
}
