package geecache

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/nukecoke1828/7daysProgram/GeeCache/geecache/consistenthash"
	pb "github.com/nukecoke1828/7daysProgram/GeeCache/geecache/geecachepb"
)

const (
	defaultBasePath = "/_geecache/" // 默认的HTTP请求路径前缀
	defaultReplicas = 50            // 默认的虚拟节点副本数量（用于一致性哈希）
)

// 接口实现验证（编译时检查）
var (
	_ PeerGetter = (*httpGetter)(nil) // 确保httpGetter实现了PeerGetter接口
	_ PeerPicker = (*HTTPPool)(nil)   // 确保HTTPPool实现了PeerPicker接口
)

// HTTPPool 实现了一个HTTP服务器池，用于提供分布式缓存服务
// 每个节点都运行此服务，其他节点可以通过HTTP访问其缓存数据
type HTTPPool struct {
	self        string                 // 当前节点的地址（格式为"host:port"，如"localhost:8000"）
	basePath    string                 // HTTP请求路径前缀（默认为"/_geecache/"）
	mu          sync.Mutex             // 保护peers和httpGetters的互斥锁
	peers       *consistenthash.Map    // 一致性哈希映射，用于节点选择
	httpGetters map[string]*httpGetter // 节点地址到对应httpGetter的映射
}

// httpGetter 实现PeerGetter接口，用于向其他节点发送HTTP请求获取缓存
type httpGetter struct {
	baseURL string // 基础URL格式：节点地址 + basePath（如"http://localhost:8000/_geecache/"）
}

// NewHTTPPool 创建并返回一个新的HTTPPool实例
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
		http.Error(w, "HTTPPool serving unexpected path: "+r.URL.Path, http.StatusBadRequest)
		return
	}

	// 记录请求日志
	p.Log("%s %s", r.Method, r.URL.Path)

	// 2. 提取组名和键
	// 示例路径：/_geecache/scores/Tom → ["scores", "Tom"]
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path: expected format /<group>/<key>", http.StatusBadRequest)
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

	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. 返回二进制数据
	w.Header().Set("Content-Type", "application/octet-stream") // 二进制流
	w.Write(body)                                              // 写入响应体
}

// Get 实现PeerGetter接口，向指定节点发送HTTP GET请求获取缓存值
// group: 缓存组名
// key: 缓存键
// 返回: 缓存值或错误
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	// 构建URL：baseURL/group/key（对组名和键进行URL编码）
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()), // 编码组名（处理特殊字符）
		url.QueryEscape(in.GetKey()),   // 编码键（处理特殊字符）
	)

	// 发送HTTP GET请求
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 检查HTTP状态码
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %v", res.StatusCode)
	}

	// 读取响应体
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}

// Set 初始化节点池并设置一致性哈希环
// peers: 所有节点的地址列表（包括当前节点）
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 创建一致性哈希映射（使用默认副本数）
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 添加所有节点到哈希环
	p.peers.Add(peers...)

	// 为每个节点创建httpGetter
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		// 为每个节点创建访问器（基础URL = 节点地址 + 基础路径）
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer 实现PeerPicker接口，根据键选择对应的节点
// key: 要查询的缓存键
// 返回: 选择的节点访问器（PeerGetter）和是否成功找到
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 使用一致性哈希选择节点
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer) // 记录节点选择
		return p.httpGetters[peer], true
	}

	// 如果选择的是当前节点或未找到节点，返回nil
	return nil, false
}
