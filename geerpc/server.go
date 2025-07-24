// Package geerpc 是 7 天实现 RPC 框架的核心包。
// 主要负责 RPC 服务端的核心逻辑，包括协议握手、编解码、请求处理与响应发送。
package geerpc

import (
	"encoding/json" // JSON 编解码
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/nukecoke1828/7daysProgram/geerpc/codec"
)

// MagicNumber 是通信双方用来“握手”的魔数。
// 客户端在建立连接后首先发送该数字，服务端收到后验证是否匹配，
// 以此确认对方使用的是本框架的协议。
const MagicNumber = 0x3bef5c

const (
	connected        = "200 Connected to Gee RPC"
	defaultRPCPath   = "/_geeprc_"
	defaultDebugPath = "/debug/geerpc"
)

// DefaultOption 是默认的协议选项实例。
// 默认使用 MagicNumber 作为魔数，并使用 GobType 作为编解码格式。
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: 10 * time.Second, // 默认连接超时 10 秒
}

// DefaultServer 是全局默认的 RPC 服务端实例，便于直接调用 Accept 方法。
var DefaultServer = NewServer()

// invalidRequest 是一个占位符，用于在请求非法时作为响应体。
var invalidRequest = struct{}{}

// Option 定义了建立连接时需要协商的协议选项。
type Option struct {
	MagicNumber    int        // 魔数，用于协议识别
	CodecType      codec.Type // 选定的编解码类型（如 Gob、JSON）
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
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
		conn, err := lis.Accept() // 阻塞等待客户端连接
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
		go s.handleRequest(cc, req, sending, wg, DefaultOption.HandleTimeout)
	}
	// 等待所有 goroutine 完成后关闭连接，防止未完成就被关闭
	wg.Wait()
	_ = cc.Close()
}

// readRequestHeader 从连接中读取并解码请求头。
func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF { // 非正常结束，打印错误日志
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
	if err = cc.ReadBody(argvi); err != nil { // 将请求体反序列化到 argv
		log.Println("rpc server: read body error:", err)
		return nil, err
	}
	return req, nil
}

// sendResponse 发送响应，使用互斥锁保证并发安全。
func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock() // 确保响应按顺序写入连接
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil { // 将响应序列化并写入连接
		log.Println("rpc server: write response error:", err)
	}
}

// handleRequest 处理单个请求并发送响应。
func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	// 通道使用struct类型0内存占用，同时防止误用
	called := make(chan struct{}) // 业务方法执行完成的信号
	sent := make(chan struct{})   // 响应数据已写入连接的信号
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{} // 通知调用完成
		if err != nil {      // 调用失败，发送错误响应
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{} // 通知响应已发送
			return
		}
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{} // 通知响应已发送
	}()
	if timeout == 0 { // 无超时控制，直接等待调用完成
		<-called // 等待调用完成
		<-sent   // 等待响应发送完成
		return
	}
	select {
	case <-time.After(timeout): // 超时，取消调用
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		s.sendResponse(cc, req.h, invalidRequest, sending)
	case <-called: // 调用完成
		<-sent // 等待响应发送完成
	}
}

// Register 将某个对象导出为 RPC 服务。
// 通过反射解析 rcvr 类型，生成 *service 并注册到 serviceMap
func (s *Server) Register(rcvr interface{}) error {
	server := newService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(server.name, server); dup { // 判断服务是否已注册
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
	if dot < 0 { // 格式错误
		err = errors.New("rpc: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName) // 从 serviceMap 中查找服务
	if !ok {
		err = errors.New("rpc: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)          // 得到服务实例
	mtype = svc.method[methodName] // 得到对应方法类型
	if mtype == nil {
		err = errors.New("rpc: can't find method " + methodName + " in service " + serviceName)
	}
	return
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" { // 如果不是 CONNECT 请求，返回 405
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack() // 从HTTP服务器获取TCP连接
	if err != nil {
		log.Print("rpc hijacking ", r.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n") // 向客户端发送 HTTP 200 响应
	s.ServeConn(conn)
}

// HandleHTTP 注册 HTTP 处理函数，处理 RPC 请求。
func (s *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, s)
	http.Handle(defaultDebugPath, debugHTTP{s})
	log.Println("rpc server debug path:", defaultDebugPath)
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
