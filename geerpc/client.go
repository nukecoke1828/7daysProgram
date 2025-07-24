// Package geerpc 是一个极简 RPC 框架的客户端实现
// 支持异步/同步调用、连接管理、请求-响应匹配、并发安全等核心能力
package geerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nukecoke1828/7daysProgram/geerpc/codec"
)

// 确保 Client 实现了 io.Closer 接口
var _ io.Closer = (*Client)(nil)

// ErrShutdown 是当连接已关闭或正在关闭时返回的统一错误
var ErrShutdown = errors.New("connection is shut down")

type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

// Call 代表一次 RPC 调用
type Call struct {
	Seq           uint64      // 请求唯一序号，用于匹配响应
	ServiceMethod string      // 远程服务方法名，格式 "Service.Method"
	Args          interface{} // 参数
	Reply         interface{} // 结果指针（由用户传入）
	Error         error       // 错误信息
	Done          chan *Call  // 调用结束通知通道，收到 *Call 即表示完成
}

// Client 是一个 RPC 客户端连接实例
type Client struct {
	cc       codec.Codec      // 编解码器（Gob/JSON…）
	opt      *Option          // 协议选项
	sending  sync.Mutex       // 保证写请求的串行化（避免乱序）
	header   codec.Header     // 复用的请求头（减少内存分配）
	mu       sync.Mutex       // 保护客户端状态的互斥锁
	seq      uint64           // 下一个待分配的请求序号
	pending  map[uint64]*Call // 记录已发送未完成的请求
	closing  bool             // 用户主动调用 Close 时为 true
	shutdown bool             // 服务端/网络错误导致不可用时为 true
}

type clientResult struct {
	client *Client
	err    error
}

// done 把 call 发送到 Done 通道，通知调用方请求已完成
func (c *Call) done() { c.Done <- c }

// Close 优雅关闭客户端连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing { // 防止重复关闭
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// IsAvailable 判断连接是否仍可用
func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closing && !c.shutdown
}

// registerCall 把新的调用注册到 pending 映射，并分配唯一序号
func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.pending[c.seq] = call
	c.seq++
	return call.Seq, nil
}

// removeCall 根据序号移除并返回对应 Call
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// terminateCalls 在连接异常/关闭时，将所有未完成的调用标记错误并结束
func (c *Client) terminateCalls(err error) {
	c.sending.Lock() // 先锁发送再锁状态，确保顺序一致
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

// receive 在后台 goroutine 中持续读取服务端响应
// 根据 Seq 找到对应 Call，填充 Reply/Error，并通过 Call.done() 通知
func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil {
			break // 读头失败，跳出循环
		}
		call := c.removeCall(h.Seq) // 找到对应调用
		switch {
		case call == nil:
			// 序号不存在：可能是超时已被移除，丢弃响应体
			err = c.cc.ReadBody(nil)
		case h.Error != "":
			// 服务端返回错误
			call.Error = fmt.Errorf("%s", h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			// 读取正确结果
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body error: " + err.Error())
			}
			call.done()
		}
	}
	// 出现任何读取错误，结束所有挂起调用
	c.terminateCalls(err)
}

// NewClient 基于已建立的连接和 Option 创建并初始化客户端
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// 先把 Option 以 JSON 形式写给服务端做握手
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error:", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

// newClientCodec 创建客户端实例并启动 receive goroutine
func newClientCodec(cc codec.Codec, opt *Option) *Client {
	client := &Client{
		seq:     1, // 序号从 1 开始
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive() // 后台持续读取响应
	return client
}

// parseOptions 解析用户传入的可变 Option，填充默认值
func parseOptions(opts ...*Option) (*Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 { // 最多只能有一个 Option
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber // 强制魔数
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType // 默认 Gob
	}
	return opt, nil
}

// Dial 便捷函数：拨号、握手并返回已就绪的客户端
// func Dial(network, address string, opts ...*Option) (client *Client, err error) {
// 	opt, err := parseOptions(opts...)
// 	if err != nil {
// 		return nil, err
// 	}
// 	conn, err := net.Dial(network, address)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// 发生错误时自动关闭连接
// 	defer func() {
// 		if err != nil {
// 			_ = conn.Close()
// 		}
// 	}()
// 	return NewClient(conn, opt)
// }

// send 将 Call 包装成请求并写入连接；出错时立即完成调用
func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	// 注册到 pending，获取序号
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 填充并发送请求头 + 参数
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = seq
	c.header.Error = ""
	if err := c.cc.Write(&c.header, call.Args); err != nil {
		// 发送失败，移除并通知
		call = c.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go 发起一次异步调用
// 如果 done == nil，自动创建带缓冲通道；否则要求通道必须有缓冲
func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10) // 默认缓冲 10，避免调用方阻塞
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call) // 异步发送
	return call  // 立即返回 *Call，调用方可从 call.Done 接收结果
}

// Call 同步调用：内部使用 Go 发起异步调用，并阻塞等待结果
// 由 context.Context 控制超时或取消
func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 创建带缓冲的通道，确保不会阻塞异步调用
	done := make(chan *Call, 1)
	call := c.Go(serviceMethod, args, reply, done)

	select {
	case <-ctx.Done(): // 超时或取消(channel被关闭)
		c.removeCall(call.Seq)
		return fmt.Errorf("rpc client: call timeout: %w", ctx.Err())
	case call = <-done: // 调用完成
		return call.Error
	}
}

func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil { // 连接失败，直接返回错误
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close() // 发生错误时关闭连接
		}
	}()
	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt) // 获取客户端实例
		if err != nil {
			log.Fatal("rpc client: dialTimeout:", err)
		}
		ch <- clientResult{client: client, err: err}
	}()
	if opt.ConnectTimeout == 0 { // 不设置超时，直接返回结果
		result := <-ch
		return result.client, result.err
	}
	select { // 超时控制
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc client: connect timeout:expect within %s", opt.ConnectTimeout)
	case result := <-ch:
		return result.client, result.err
	}
}

func Dial(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewClient, network, address, opts...)
}

func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", defaultRPCPath, "geerpc")
	_, err := io.WriteString(conn, req) // 将CONNECT请求发送给服务端
	if err != nil {
		log.Fatal("rpc client: io:", err)
	}
	// 按HTTP协议，读取服务端响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil {
		log.Fatal("rpc client: NewHTTPClient:", err)
	}
	if err == nil && resp.Status == connected { // 连接成功
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}
