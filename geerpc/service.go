package geerpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// methodType 描述了一个 RPC 方法的完整元数据
type methodType struct {
	method    reflect.Method // 通过反射得到的 *方法* 本身
	ArgType   reflect.Type   // 第 2 个入参的类型（请求结构体）
	ReplyType reflect.Type   // 第 3 个入参的类型（响应结构体）
	numCalls  uint64         // 被调用的总次数（原子计数，线程安全）
}

// service 描述了一个 RPC 服务（即一个对象）的全部信息
type service struct {
	name   string                 // 服务名称，等于接收者类型的名字
	typ    reflect.Type           // 接收者的反射类型
	rcvr   reflect.Value          // 接收者的反射值
	method map[string]*methodType // 该服务暴露的所有方法，key 为方法名
}

// NumCalls 返回该 RPC 方法被调用的总次数（原子读取）
func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

// newArgv 根据 ArgType 创建一个新的请求参数实例，并返回其 reflect.Value
// 如果 ArgType 是指针类型，返回指向新实例的指针；否则返回值本身
func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr { // 指针类型
		// 参数本身是指针：new(T) 返回 *T
		argv = reflect.New(m.ArgType.Elem())
	} else { // 值类型
		// 参数是值类型：new(T) 返回 *T，再取其元素拿到 T
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

// newReplyv 根据 ReplyType 创建一个新的响应参数实例
// 对于 map、slice 等引用类型额外做一次初始化，避免后续空指针
func (m *methodType) newReplyv() reflect.Value {
	// ReplyType 必须是 *T 形式，先 new(T) 拿到 *T
	replyv := reflect.New(m.ReplyType.Elem())

	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		// map 需要 make
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		// slice 需要 make，长度 0
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

// newService 使用用户提供的接收者对象构造一个 *service
// rcvr代表一个实现了 RPC 服务的对象，必须是指针类型，否则无法获取其方法
func newService(rcvr interface{}) *service {
	s := new(service)

	// 保存接收者对象的反射值
	s.rcvr = reflect.ValueOf(rcvr)

	// 服务名 = 接收者类型的名字（去掉指针层级）
	s.name = reflect.Indirect(s.rcvr).Type().Name()

	// 记录类型信息
	s.typ = reflect.TypeOf(rcvr)

	// 类型名必须导出，否则无法被包外调用
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}

	// 扫描并注册所有符合规范的方法
	s.registerMethods()
	return s
}

// registerMethods 遍历接收者类型的所有方法，将符合 RPC 规范的方法注册到 service.method
func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)

	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mtype := method.Type

		// 方法签名的硬性要求：
		// func (t *T) MethodName(arg, reply interface{}) error
		if mtype.NumIn() != 3 || mtype.NumOut() != 1 {
			continue
		}
		// 返回类型必须是 error
		// reflect.TypeOf需要的是一个合法的值或表达式，所以需要用(*error)(nil)
		if mtype.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}

		// 第 2、3 个入参必须是导出或内置类型
		argType, replyType := mtype.In(1), mtype.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}

		// 方法通过所有检查，加入映射
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s", s.name, method.Name)
	}
}

// isExportedOrBuiltinType 判断类型是否是导出的结构体/接口，或者是内置类型（如 int、[]byte）
func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

// call 真正执行一次 RPC 方法调用
// argv、replyv 已经通过 newArgv/newReplyv 构造并解码完成
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	// 原子增加调用次数
	atomic.AddUint64(&m.numCalls, 1)

	// 取出方法对应的函数值
	f := m.method.Func

	//在反射层面执行函数调用必须调用Call方法，参数和返回值 必须用[]reflect.Value包装/解包
	// 调用方法：rcvr.Method(argv, replyv）
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})

	// 方法返回值列表中第 0 个必须是 error
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}
