// Package codec 定义了用于 RPC 通信的通用编解码接口和实现。
// 支持多种数据格式（如 Gob、JSON）的序列化与反序列化。
package codec

import "io"

// 定义支持的编码类型常量，用于标识不同的编解码格式。
const (
	GobType  Type = "application/gob"  // Gob 编码类型标识
	JsonType Type = "application/json" // JSON 编码类型标识
)

// NewCodecFuncMap 是一个映射表，用于根据编码类型获取对应的编解码器构造函数。
// 在 init 函数中初始化并注册支持的编解码器。
var NewCodecFuncMap map[Type]NewCodecFunc

// Type 表示编解码的类型，本质是一个字符串，用于标识不同的数据格式。
type Type string

// NewCodecFunc 是一个函数类型，定义了创建编解码器实例的函数签名。
// 参数 io.ReadWriteCloser 是一个可读可写可关闭的连接（如网络连接）。
type NewCodecFunc func(io.ReadWriteCloser) Codec

// Header 是 RPC 通信中消息头的结构体，用于传输元数据。
type Header struct {
	ServiceMethod string // 服务方法名，格式为 "Service.Method"，用于定位服务端对应的方法
	Seq           uint64 // 客户端请求的唯一序列号，用于匹配响应
	Error         string // 服务端返回的错误信息（如果有）
}

// Codec 是一个接口，定义了所有编解码器必须实现的方法。
// 它同时继承了 io.Closer，表示编解码器可以被关闭。
type Codec interface {
	io.Closer
	ReadHeader(*Header) error         // 从连接中读取消息头
	ReadBody(interface{}) error       // 从连接中读取消息体（实际参数或返回值）
	Write(*Header, interface{}) error // 将消息头和消息体写入连接
}

// init 函数在包被导入时自动执行，用于初始化 NewCodecFuncMap。
// 它注册了默认支持的 Gob 编解码器。
func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec // 注册 Gob 类型的编解码器构造函数
}
