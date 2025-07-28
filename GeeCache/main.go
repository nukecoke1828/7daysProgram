package main

import (
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

func main() {
	// 1. 创建缓存组 "scores"
	geecache.NewGroup(
		"scores", // 缓存组名称
		2<<10,    // 缓存容量 2048字节 (2^11)
		geecache.GetterFunc(func(key string) ([]byte, error) {
			// 数据源获取函数（缓存未命中时调用）
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}),
	)

	// 2. 创建HTTP缓存服务
	addr := "localhost:9999"            // 服务监听地址
	peers := geecache.NewHTTPPool(addr) // 创建HTTP服务实例

	// 3. 启动HTTP服务器
	log.Println("geecache is running at", addr)

	// 启动服务（阻塞运行）
	// 访问示例: http://localhost:9999/_geecache/scores/Tom
	log.Fatal(http.ListenAndServe(addr, peers))
}
