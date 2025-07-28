package geecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultBasePath = "/_geecache/" // 默认的HTTP请求路径前缀

// HTTPPool 实现了一个HTTP服务器池，用于提供分布式缓存服务
// 每个节点都运行此服务，其他节点可以通过HTTP访问其缓存数据
type HTTPPool struct {
	self     string // 当前节点的地址（如"localhost:8000"）
	basePath string // HTTP请求路径前缀（默认为"/_geecache/"）
}

// NewHTTPPool 创建一个新的HTTPPool实例
// self: 当前节点的网络地址（如"localhost:8000"）
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath, // 使用默认路径前缀
	}
}

// Log 提供带节点标识的日志记录功能
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP 实现http.Handler接口，处理HTTP请求
// 请求路径格式：/[basePath]/[groupName]/[key]
// 处理流程：验证路径 → 提取组名和键 → 获取缓存组 → 查询键值 → 返回结果
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 验证请求路径前缀
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}

	// 记录请求日志
	p.Log("%s %s", r.Method, r.URL.Path)

	// 2. 提取组名和键
	// 示例路径：/_geecache/scores/Tom → ["scores", "Tom"]
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path: expected format /<group>/<key>",
			http.StatusBadRequest)
		return
	}

	groupName := parts[0] // 缓存组名（如"scores"）
	key := parts[1]       // 缓存键（如"Tom"）

	// 3. 获取缓存组
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	// 4. 从缓存组获取值
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. 返回二进制数据
	w.Header().Set("Content-Type", "application/octet-stream") // 二进制流
	w.Write(view.ByteSlice())                                  // 写入响应体
}
