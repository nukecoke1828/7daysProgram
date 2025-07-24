// Package xclient 提供一个支持负载均衡、连接复用、并发广播调用的 RPC 客户端封装。
package xclient

import (
	"context"
	"io"
	"reflect"
	"sync"

	. "github.com/nukecoke1828/7daysProgram/geerpc"
)

// 编译期断言：确保 *XClient 实现了 io.Closer 接口。
var _ io.Closer = (*XClient)(nil)

// XClient 是对 geerpc.Client 的增强封装：
//   - 通过 Discovery 获取可用服务地址（负载均衡）。
//   - 缓存每个地址对应的 *Client，避免重复建连。
//   - 支持普通单点调用 (Call) 以及并发广播调用 (Broadcast)。
type XClient struct {
	d       Discovery          // 服务发现接口，负责给出可用节点
	mode    SelectMode         // 负载均衡策略（Random、RoundRobin…）
	opt     *Option            // 全局 RPC 配置（编解码、超时等）
	mu      sync.Mutex         // 保护 clients 并发读写
	clients map[string]*Client // 地址 -> *Client 的本地连接池
}

// NewXClient 创建一个新的 XClient 实例。
// 参数：
//
//	d    : 服务发现实例
//	mode : 负载均衡策略
//	opt  : RPC 选项
func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

// Close 关闭并清理所有已缓存的 RPC 连接。
// 实现 io.Closer 接口，可在程序退出时统一调用。
func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients {
		_ = client.Close() // 关闭底层 TCP 连接
		delete(xc.clients, key)
	}
	return nil
}

// dial 根据 rpcAddr 获取一个可用的 *Client：
//   - 先查本地缓存，如果已存在且健康，直接复用。
//   - 如果不可用或不存在，则新建连接并放入缓存。
//
// 内部加锁保证并发安全。
func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	// 1. 缓存命中，检查健康度
	if client, ok := xc.clients[rpcAddr]; ok {
		if client.IsAvailable() {
			return client, nil
		}
		// 不可用，关闭并删除
		_ = client.Close()
		delete(xc.clients, rpcAddr)
	}

	// 2. 缓存未命中或已删除，重新建连
	client, err := XDial(rpcAddr, xc.opt) // 内部根据 opt 创建 geerpc.Client
	if err != nil {
		return nil, err
	}
	xc.clients[rpcAddr] = client
	return client, nil
}

// call 在指定节点上执行单次 RPC 调用。
// 封装了 dial + client.Call，方便复用。
func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// Call 根据负载均衡策略选择单个节点，然后在该节点上执行 RPC。
// 对用户暴露的“单点调用”入口。
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 1. 使用 Discovery 和负载均衡策略选出一个地址
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	// 2. 在该地址上执行调用
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

// Broadcast 并发地向所有服务节点发起同一 RPC 调用。
// 规则：
//   - 任意节点返回成功即把结果写入 reply，后续成功不再覆盖。
//   - 记录第一个出现的错误，并立即通过 context.WithCancel 取消其余调用。
//   - 如果 reply==nil，表示无需收集返回值，仅做“通知”式广播。
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 1. 获取全部节点
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error // 记录第一个错误

	// replyDone == true 表示已收到首个成功响应，后续不再写 reply
	replyDone := reply == nil

	// 创建可取消的 context，出现首个错误时通知所有 goroutine 退出
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 2. 为每个节点启动 goroutine
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()

			// 为本次调用克隆一份 reply 实例，避免并发写冲突
			var cloneReply interface{}
			if reply != nil {
				// 通过反射创建与原始 reply 同类型的新零值
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			// 3. 发起 RPC
			err := xc.call(rpcAddr, ctx, serviceMethod, args, cloneReply)

			// 4. 在临界区内更新共享变量
			mu.Lock()
			defer mu.Unlock()

			if err != nil && e == nil { // 第一个错误
				e = err
				// 将当前失败的 reply（可能部分有效）写回，便于调试
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				cancel() // 通知其余 goroutine 提前退出
			}

			if err == nil && !replyDone { // 第一个成功
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
		}(rpcAddr)
	}

	// 5. 等待所有 goroutine 结束
	wg.Wait()
	return e // 返回第一个遇到的错误（可能为 nil）
}
