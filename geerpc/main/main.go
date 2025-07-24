// main.go 演示了一个完整的 RPC 使用流程：
// 1. 启动注册中心
// 2. 启动 2 个 RPC 服务并注册到注册中心
// 3. 通过服务发现 + 负载均衡进行单点调用和广播调用
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/nukecoke1828/7daysProgram/geerpc"          // RPC 框架
	"github.com/nukecoke1828/7daysProgram/geerpc/registry" // 注册中心
	"github.com/nukecoke1828/7daysProgram/geerpc/xclient"  // 带负载均衡的客户端
)

// Foo 是我们要暴露的 RPC 服务类型
type Foo int

// Args 用于 Sum 和 Sleep 方法的参数
type Args struct {
	Num1, Num2 int
}

// ------------------------------------------------------------------
// 1) 传统方式：直接启动 RPC Server（仅用于示例 call() 函数）
// ------------------------------------------------------------------

// startServer 启动一个固定端口 :9999 的 RPC 服务
// 并把监听地址写入 addr channel，供测试函数拿到
func startServer(addr chan string) {
	var foo Foo
	// 将 Foo 的方法注册到默认 RPC Server
	if err := geerpc.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
	// 监听本地任意端口
	l, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())

	// 先把地址丢给 caller，再真正开始服务
	addr <- l.Addr().String()

	// 把 RPC Server 注册到 HTTP 的 DefaultServeMux
	geerpc.HandleHTTP()
	_ = http.Serve(l, nil)
}

// Sum 是 Foo 的一个 RPC 方法，实现 a+b
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// ------------------------------------------------------------------
// 2) 传统方式：客户端直连测试
// ------------------------------------------------------------------

// call 使用传统 DialHTTP 方式连接 startServer 启动的服务
func call(addrCh chan string) {
	log.Println("dial")
	// 阻塞等待 startServer 把地址写进来
	client, err := geerpc.DialHTTP("tcp", <-addrCh)
	if err != nil {
		log.Fatalf("dialhttp error: %v", err)
	}
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second) // 给服务端一点启动时间

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		// 传入 i 避免闭包陷阱
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: i, Num2: i * i}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i) // 将i作为参数传入匿名函数
	}
	wg.Wait()
}

// ------------------------------------------------------------------
// 3) 注册中心方式：启动多个 Server 并注册到中心
// ------------------------------------------------------------------

// startServer2 启动一个随机端口的 RPC Server，并向注册中心定时心跳
func startServer2(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	// 监听随机端口
	l, _ := net.Listen("tcp", ":0")
	server := geerpc.NewServer()
	_ = server.Register(&foo)

	// 向注册中心发送心跳（第二个参数为服务地址，格式如 "tcp@127.0.0.1:12345"）
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)

	wg.Done() // 通知 main 本服务已就绪
	server.Accept(l)
}

// Sleep 是 Foo 的另一个 RPC 方法，用于广播演示
func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

// ------------------------------------------------------------------
// 4) 带注册中心的高级客户端调用示例
// ------------------------------------------------------------------

// foo 是对 xclient 的统一封装：支持单点 call 和 broadcast
func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

// call2 使用服务发现 + 随机负载均衡的单点调用
func call2(registry string) {
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

// broadcast 使用服务发现 + 并发广播调用
func broadcast(registry string) {
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

// ------------------------------------------------------------------
// 5) 启动注册中心
// ------------------------------------------------------------------

// startRegistry 启动一个 HTTP 注册中心，监听固定端口 :9999
func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP() // 注册 HTTP 路由
	wg.Done()
	_ = http.Serve(l, nil)
}

// ------------------------------------------------------------------
// 6) main：编排所有流程
// ------------------------------------------------------------------

func main() {
	log.SetFlags(0) // 去掉时间前缀，简洁输出

	// 注册中心固定地址
	registryAddr := "http://localhost:9999/_geerpc_/registry"

	var wg sync.WaitGroup

	// 1) 先启动注册中心
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()
	time.Sleep(time.Second) // 等注册中心就绪

	// 2) 启动 2 个 RPC 服务，并注册到中心
	wg.Add(2)
	go startServer2(registryAddr, &wg)
	go startServer2(registryAddr, &wg)
	wg.Wait()
	time.Sleep(time.Second) // 等服务注册完成

	// 3) 通过服务发现进行调用
	call2(registryAddr)     // 单点调用
	broadcast(registryAddr) // 广播调用
}
