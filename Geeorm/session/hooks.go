package session

import (
	"reflect"

	"github.com/nukecoke1828/7daysProgram/Geeorm/log"
)

const (
	BeforeQuery  = "BeforeQuery"
	AfterQuery   = "AfterQuery"
	BeforeUpdate = "BeforeUpdate"
	AfterUpdate  = "AfterUpdate"
	BeforeDelete = "BeforeDelete"
	AfterDelete  = "AfterDelete"
	BeforeInsert = "BeforeInsert"
	AfterInsert  = "AfterInsert"
)

// 反射钩子触发器(钩子方法定义在「表模型结构体」（或传入的任意对象）)
func (s *Session) CallMethod(method string, value interface{}) {
	fm := reflect.ValueOf(s.RefTable().Model).MethodByName(method) // 从结构体中获取钩子方法
	if value != nil {                                              // 如果有传入具体对象
		fm = reflect.ValueOf(value).MethodByName(method) // 从具体对象中获取钩子方法
	}
	param := []reflect.Value{reflect.ValueOf(s)} // *Session作为唯一参数传入
	if fm.IsValid() {                            // 如果钩子方法存在
		if v := fm.Call(param); len(v) > 0 { // 调用钩子方法
			if err, ok := v[0].Interface().(error); ok { // 如果钩子方法返回了error打印日志
				log.Error(err)
			}
		}
	}
}
