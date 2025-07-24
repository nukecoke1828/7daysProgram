package geerpc

import (
	"fmt"
	"reflect"
	"testing"
)

// Foo 是一个占位类型，用于演示 RPC 服务
// 底层用 int，但这里并不真正用到它的值
type Foo int

// Args 定义了 Sum 方法的请求结构体
type Args struct {
	Num1, Num2 int
}

// Sum 是一个导出的方法，满足 RPC 方法签名：
//
//	func (t *T) MethodName(args T1, reply *T2) error
//
// 因此会被注册到 service.method 中
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// sum 是非导出方法，不满足 ast.IsExported 检查，不会被注册
func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// _assert 是一个简单的断言辅助函数
// 如果 condition 为 false，则 panic 并输出格式化信息
func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

// TestNewService 测试 newService 能否正确扫描并注册方法
// 期望只注册到 Sum 这一个方法
func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)

	// 断言只扫描到 1 个方法
	_assert(len(s.method) == 1, "wrong service Method, expect 1 got %d", len(s.method))

	// 断言 Sum 方法存在
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong Method, Sum shouldn't be nil")
}

// TestMethodType_Call 测试通过反射调用已注册的 RPC 方法
func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)

	// 取出 Sum 方法的元数据
	mType := s.method["Sum"]

	// 利用 newArgv/newReplyv 构造两个参数实例
	argv := mType.newArgv()     // 类型为 Args
	replyv := mType.newReplyv() // 类型为 *int

	// 为 argv 设置实际值：Args{1, 3}
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 3}))

	// 通过 service.call 真正执行 Foo.Sum
	err := s.call(mType, argv, replyv)

	// 断言：
	// 1. 无错误
	// 2. 计算结果正确 (*replyv == 4)
	// 3. 调用次数被正确累加 (NumCalls == 1)
	_assert(err == nil &&
		*replyv.Interface().(*int) == 4 &&
		mType.NumCalls() == 1,
		"failed to call Foo.Sum")
}
