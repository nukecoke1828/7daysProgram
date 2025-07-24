package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultPath    = "/_geerpc_/registry"
	defaultTimeout = 5 * time.Minute
)

var DefaultGeeRegistry = New(defaultTimeout)

type GeeRegistry struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr  string
	start time.Time
}

func New(timeout time.Duration) *GeeRegistry {
	return &GeeRegistry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

func (r *GeeRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()} // 如果不存在，则新建
	} else {
		s.start = time.Now() // 如果存在，则更新时间
	}
}

func (r *GeeRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) { // 如果超时时间为0，则永久存活；否则，计算存活时间，并判断是否存活
			alive = append(alive, addr)
		} else { // 超时，则删除
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive) // 按字典顺序排序
	return alive
}

func (r *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET": // 返回存活的节点
		w.Header().Set("X-Geerpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST": // 注册节点
		addr := req.Header.Get("X-Geerpc-Server") // 根据请求头获取地址
		if addr == "" {                           // 如果没有提供地址，则返回错误
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default: // 其他方法不允许
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *GeeRegistry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r) // 注册到默认路径
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultGeeRegistry.HandleHTTP(defaultPath)
}

func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 { // 如果超时时间为0，则使用默认超时时间
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr) // 第一次发送心跳用于注册
	go func() {                         // 定时发送心跳
		t := time.NewTicker(duration) // 时间间隔为duration
		for err == nil {
			<-t.C // 阻塞goroutine，等待duration时间
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heartbeat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-Geerpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil { // 发送心跳请求
		log.Println("rpc server: heartbeart error:", err)
		return err
	}
	return nil
}
