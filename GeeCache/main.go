package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/nukecoke1828/7daysProgram/GeeCache/geecache"
)

// 模拟数据库（内存数据源）
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// createGroup 创建并返回一个缓存组
func createGroup() *geecache.Group {
	// 创建名为"scores"的缓存组，容量为2KB
	// 使用GetterFunc提供数据源访问逻辑
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key) // 模拟慢查询日志

			// 从模拟数据库查询
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key) // 键不存在错误
		}))
}

// startCacheServer 启动缓存节点服务器
// addr: 当前节点地址（如"http://localhost:8001"）
// addrs: 所有节点地址列表
// gee: 缓存组实例
func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	// 1. 创建HTTP节点池
	peers := geecache.NewHTTPPool(addr)

	// 2. 设置所有节点（包括当前节点）
	peers.Set(addrs...)

	// 3. 将节点选择器注册到缓存组
	gee.RegisterPeers(peers)

	log.Println("cache is running at", addr)

	// 4. 启动HTTP服务器
	// 注意：地址格式转换（去掉"http://"前缀）
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

// startAPIServer 启动API网关服务器
// apiAddr: API服务器地址（如"http://localhost:9999"）
// gee: 缓存组实例
func startAPIServer(apiAddr string, gee *geecache.Group) {
	// 注册API路由
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// 1. 获取查询参数中的键
			key := r.URL.Query().Get("key")

			// 2. 从缓存组获取值
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// 3. 返回二进制数据
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())
		}))

	log.Println("api server is running at", apiAddr)

	// 启动HTTP服务器
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func main() {
	// 1. 解析命令行参数
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "cache server port")
	flag.BoolVar(&api, "api", false, "start a api server?")
	flag.Parse()

	// 2. 配置服务地址
	apiAddr := "http://localhost:9999" // API网关地址
	addrMap := map[int]string{         // 缓存节点地址映射
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	// 3. 准备所有节点地址列表
	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	// 4. 创建缓存组
	gee := createGroup()

	// 5. 如果启用API模式，启动API服务器
	if api {
		go startAPIServer(apiAddr, gee) // 在goroutine中运行
	}

	// 6. 启动缓存节点服务器
	startCacheServer(addrMap[port], addrs, gee)
}
