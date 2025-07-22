// Package geerpc 是 7 天实现 RPC 框架的核心包。
// 主要负责 RPC 服务端的核心逻辑，包括协议握手、编解码、请求处理与响应发送。
package geerpc

import (
	"encoding/json" // JSON 编解码
	"errors"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/nukecoke1828/7daysProgram/geerpc/codec"
)

// MagicNumber 是通信双方用来“握手”的魔数。
// 客户端在建立连接后首先发送该数字，服务端收到后验证是否匹配，
// 以此确认对方使用的是本框架的协议。
const MagicNumber = 0x3bef5c

// DefaultOption 是默认的协议选项实例。
// 默认使用 MagicNumber 作为魔数，并使用 GobType 作为编解码格式。
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// DefaultServer 是全局默认的 RPC 服务端实例，便于直接调用 Accept 方法。
var DefaultServer = NewServer()

// invalidRequest 是一个占位符，用于在请求非法时作为响应体。
var invalidRequest = struct{}{}

// Option 定义了建立连接时需要协商的协议选项。
type Option struct {
	MagicNumber int        // 魔数，用于协议识别
	CodecType   codec.Type // 选定的编解码类型（如 Gob、JSON）
}

// Server 表示一个 RPC 服务端实例。
type Server struct {
	serviceMap sync.Map // 线程安全地保存所有注册的服务，key=服务名，value=*service
}

// request 封装一次 RPC 请求的所有信息。
type request struct {
	h            *codec.Header // 请求头
	argv, replyv reflect.Value // 请求参数和响应值的反射表示
	mtype        *methodType   // 请求所对应的方法类型
	svc          *service      // 请求对应的服务实例
}

// NewServer 返回一个新的 Server 实例。
func NewServer() *Server {
	return &Server{}
}

// Accept 监听并接收来自 Listener 的连接，每收到一个连接就启动一个 goroutine 处理。
func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		// 每个连接独立处理，互不阻塞
		go s.ServeConn(conn)
	}
}

// Accept 是 DefaultServer 的便捷方法，直接使用全局默认实例监听。
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// ServeConn 处理单个客户端连接。
// 1. 使用 JSON 解码 Option（握手阶段）
// 2. 验证魔数
// 3. 根据 Option.CodecType 创建对应编解码器
// 4. 进入请求处理循环
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()

	// 第一步：读取并解码客户端发送的 Option
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error:", err)
		return
	}
	// 第二步：验证魔数是否正确
	if opt.MagicNumber != MagicNumber {
		log.Println("rpc server: magic number error:", opt.MagicNumber)
		return
	}
	// 第三步：根据 CodecType 获取编解码器构造函数
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	// 第四步：使用创建的编解码器进入请求处理循环
	s.serveCodec(f(conn))
}

// serveCodec 使用给定的编解码器循环读取请求、处理并发送响应。
// 使用 sync.WaitGroup 等待所有并发请求完成后再关闭连接。
func (s *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // 保证并发写响应时的顺序安全
	wg := new(sync.WaitGroup)  // 等待所有请求处理完成

	for {
		// 读取一个完整请求
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				// 读取失败且无法恢复，退出循环
				break
			}
			// 请求头已读取，但参数错误，发送错误响应
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		// 并发处理请求
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg)
	}
	// 等待所有 goroutine 完成后关闭连接，防止未完成就被关闭
	wg.Wait()
	_ = cc.Close()
}

// readRequestHeader 从连接中读取并解码请求头。
func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

// readRequest 读取完整请求：先读头，再读体，并构造 request 结构体。
func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	// 解析 ServiceMethod，找到对应服务和方法
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	// 创建参数与返回值实例
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()
	argvi := req.argv.Interface()
	// 若 argv 不是指针类型，取其指针后再传给 ReadBody
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read body error:", err)
		return nil, err
	}
	return req, nil
}

// sendResponse 发送响应，使用互斥锁保证并发安全。
func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

// handleRequest 处理单个请求并发送响应。
func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	err := req.svc.call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		s.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
	// 把业务方法执行结果返回给客户端
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

// Register 将某个对象导出为 RPC 服务。
// 通过反射解析 rcvr 类型，生成 *service 并注册到 serviceMap
func (s *Server) Register(rcvr interface{}) error {
	server := newService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(server.name, server); dup {
		return errors.New("rpc: service already defined: " + server.name)
	}
	return nil
}

// Register 使用 DefaultServer 注册服务，简化调用。
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// findService 根据 "Service.Method" 格式字符串查找对应服务和方法。
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc: can't find method " + methodName + " in service " + serviceName)
	}
	return
}
