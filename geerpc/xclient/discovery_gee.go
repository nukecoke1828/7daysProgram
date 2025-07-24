// Package xclient 实现了一个基于 HTTP 注册中心的服务发现器。
package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// 默认的更新间隔：10 秒
const defaultUpdateTimeout = time.Second * 10

// GeeRegistryDiscovery 通过 HTTP 方式向注册中心拉取可用节点列表
// 并嵌入 MultiServerDiscovery 实现负载均衡与缓存。
type GeeRegistryDiscovery struct {
	*MultiServerDiscovery               // 复用多节点发现与负载均衡能力
	registry              string        // 注册中心 URL（例如 http://localhost:9999/_geerpc）
	timeout               time.Duration // 本地缓存过期时间
	lastUpdate            time.Time     // 上次成功从注册中心更新时间
}

// NewGeeRegistryDiscovery 创建一个新的基于注册中心的发现器。
// registry：注册中心地址；timeout：缓存多久后强制刷新，传 0 则使用 10s 默认值。
func NewGeeRegistryDiscovery(registry string, timeout time.Duration) *GeeRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	return &GeeRegistryDiscovery{
		MultiServerDiscovery: NewMultiServerDiscovery([]string{}), // 初始节点列表为空
		registry:             registry,
		timeout:              timeout,
	}
}

// Update 手动把最新的节点列表写进内存缓存，并记录更新时间。
// 线程安全：内部加锁。
func (d *GeeRegistryDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	d.lastUpdate = time.Now()
	return nil
}

// Refresh 向注册中心发起 HTTP GET 拉取最新节点列表，并在需要时更新本地缓存。
// 当上次更新时间 + timeout 仍未过期时，直接返回，不会真正去注册中心请求。
func (d *GeeRegistryDiscovery) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 缓存未过期，直接返回
	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}

	log.Println("rpc registry: refresh servers from registry", d.registry)

	// 1. 发起 HTTP 请求
	resp, err := http.Get(d.registry)
	if err != nil {
		log.Println("rpc registry refresh error:", err)
		return err
	}
	defer resp.Body.Close() // 防止资源泄漏

	// 2. 从响应头 X-Geerpc-Servers 中读取逗号分隔的节点地址
	raw := resp.Header.Get("X-Geerpc-Servers")
	servers := strings.Split(raw, ",")

	// 3. 过滤空串并生成新的节点列表
	d.servers = make([]string, 0, len(servers))
	for _, s := range servers {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			d.servers = append(d.servers, trimmed)
		}
	}

	// 4. 记录更新时间
	d.lastUpdate = time.Now()
	return nil
}

// Get 根据负载均衡策略返回一个节点地址。
// 实际调用前会先尝试 Refresh，确保本地列表是最新的。
func (d *GeeRegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := d.Refresh(); err != nil {
		return "", err
	}
	return d.MultiServerDiscovery.Get(mode)
}

// GetAll 返回当前所有可用节点地址。
// 同样先刷新缓存再返回完整列表。
func (d *GeeRegistryDiscovery) GetAll() ([]string, error) {
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	return d.MultiServerDiscovery.GetAll()
}
