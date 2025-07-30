package geecache

import (
	"fmt"
	"log"
	"sync"

	pb "github.com/nukecoke1828/7daysProgram/GeeCache/geecache/geecachepb"
	"github.com/nukecoke1828/7daysProgram/GeeCache/geecache/singleflight"
)

// 全局管理所有缓存组
var (
	mu     sync.RWMutex              // 保护groups映射的读写锁
	groups = make(map[string]*Group) // 存储所有缓存组的映射表（组名->缓存组）
)

// GetterFunc 函数类型适配器，允许普通函数实现Getter接口
type GetterFunc func(key string) ([]byte, error)

// Getter 定义数据获取接口（缓存未命中时使用）
type Getter interface {
	Get(key string) ([]byte, error) // 从底层数据源获取数据
}

// Group 表示一个命名的缓存组（缓存命名空间）
type Group struct {
	name      string              // 缓存组名称（唯一标识）
	getter    Getter              // 数据获取器（缓存未命中时调用）
	mainCache cache               // 主缓存（并发安全的LRU缓存封装）
	peers     PeerPicker          // 节点选择器（用于分布式缓存）
	loader    *singleflight.Group // 单飞组（防止缓存击穿）
}

// Get 实现Getter接口，允许GetterFunc类型作为Getter
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key) // 直接调用底层函数
}

// NewGroup 创建并注册一个新的缓存组
// name: 组名（必须全局唯一）
// cacheBytes: 缓存容量（字节）
// getter: 数据获取器（不能为nil）
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter") // 防止空数据获取器
	}

	mu.Lock()         // 获取全局写锁
	defer mu.Unlock() // 确保释放锁

	// 创建新缓存组
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes}, // 初始化底层缓存
		loader:    &singleflight.Group{},         // 初始化单飞组
	}
	groups[name] = g // 注册到全局映射表
	return g
}

// GetGroup 通过名称获取已注册的缓存组
func GetGroup(name string) *Group {
	mu.RLock()          // 获取全局读锁
	defer mu.RUnlock()  // 确保释放锁
	return groups[name] // 返回对应缓存组
}

// Get 从缓存组获取数据
// key: 要查询的缓存键
// 返回值: 缓存值视图或错误
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required") // 空键检查
	}

	// 1. 尝试从本地缓存获取
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit") // 缓存命中日志
		return v, nil
	}

	// 2. 缓存未命中，加载数据
	return g.load(key)
}

// getLocally 从本地数据源获取数据并填充缓存
func (g *Group) getLocally(key string) (ByteView, error) {
	// 1. 调用用户提供的数据获取器
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err // 转发数据获取错误
	}

	// 2. 封装为不可变字节视图
	value := ByteView{b: cloneBytes(bytes)}

	// 3. 非阻塞填充缓存
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将数据添加到本地缓存
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value) // 添加到主缓存
}

// RegisterPeers 注册节点选择器（用于分布式缓存）
// peers: 实现了PeerPicker接口的对象
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once") // 防止重复注册
	}
	g.peers = peers
}

// load 数据加载方法（带单飞机制）
// 1. 尝试从远程节点获取
// 2. 失败则从本地数据源获取
func (g *Group) load(key string) (value ByteView, err error) {
	// 使用单飞机制确保相同键的请求只执行一次
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		// 1. 如果配置了分布式节点
		if g.peers != nil {
			// 选择远程节点
			if peer, ok := g.peers.PickPeer(key); ok {
				// 尝试从远程节点获取
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				// 记录远程获取失败
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}

		// 2. 从本地数据源获取（最终回退）
		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil // 类型断言获取结果
	}
	return
}

// getFromPeer 从远程节点获取数据
// peer: 实现了PeerGetter接口的远程节点
// key: 要查询的键
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{ // 构造请求
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res) // 调用远程节点获取数据
	if err != nil {
		return ByteView{}, err // 转发获取错误
	}
	return ByteView{b: res.Value}, nil // 封装为不可变字节视图
}
