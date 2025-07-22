package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/nukecoke1828/7daysProgram/geerpc" // RPC 框架
)

type Foo int

type Args struct {
	Num1, Num2 int
}

// startServer 在随机可用端口上启动 geerpc 服务器，并把实际监听地址通过通道发回
func startServer(addr chan string) {
	var foo Foo
	if err := geerpc.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
	// ":0" 让操作系统自动分配一个空闲端口
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())

	// 将监听地址写入通道，供主 goroutine 获取
	addr <- l.Addr().String()

	// 阻塞接受客户端连接，进入 geerpc 处理循环
	geerpc.Accept(l)
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func main() {
	log.SetFlags(0) // 简化日志输出格式

	// 创建通道用于接收服务器监听地址
	addr := make(chan string)
	go startServer(addr) // 后台启动服务器

	// 通过通道拿到服务器地址并建立连接
	client, _ := geerpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }() // 程序退出前关闭连接
	time.Sleep(time.Second)               // 确保服务器完全就绪

	var wg sync.WaitGroup
	// 并发发送 5 个请求
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()                     // 通知主 goroutine 该请求已完成
			args := &Args{Num1: i, Num2: i * i} // 请求参数
			var reply int                       // 服务端会把结果写入这里
			// 同步调用远程方法 "Foo.Sum"
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait() // 等待全部 5 个并发请求完成
}
