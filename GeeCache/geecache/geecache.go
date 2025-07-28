package geecache

import (
	"fmt"
	"log"
	"sync"
)

// 全局变量管理所有缓存组
var (
	mu     sync.RWMutex              // 保护groups映射的读写锁
	groups = make(map[string]*Group) // 存储所有缓存组的映射表
)

// GetterFunc 是函数类型适配器，允许普通函数实现Getter接口
type GetterFunc func(key string) ([]byte, error)

// Getter 定义数据获取接口
type Getter interface {
	Get(key string) ([]byte, error) // 从数据源获取数据的方法
}

// Group 表示一个命名的缓存组
type Group struct {
	name      string // 缓存组名称（唯一标识）
	getter    Getter // 数据获取器（缓存未命中时使用）
	mainCache cache  // 主缓存（并发安全的LRU缓存封装）
}

// Get 实现Getter接口，允许GetterFunc类型作为Getter
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key) // 直接调用函数本身
}

// NewGroup 创建并注册一个新的缓存组
// name: 组名（必须唯一）
// cacheBytes: 缓存容量（字节）
// getter: 数据获取器（不能为nil）
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter") // 防止空数据获取器
	}

	mu.Lock()         // 获取写锁
	defer mu.Unlock() // 确保释放锁

	// 创建新缓存组
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes}, // 初始化底层缓存
	}
	groups[name] = g // 注册到全局映射表
	return g
}

// GetGroup 通过名称获取已注册的缓存组
func GetGroup(name string) *Group {
	mu.RLock()          // 获取读锁
	defer mu.RUnlock()  // 确保释放锁
	return groups[name] // 返回对应缓存组
}

// Get 从缓存组获取数据
// 1. 检查缓存是否命中
// 2. 未命中时调用load方法加载数据
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required") // 空键检查
	}

	// 尝试从缓存获取
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit") // 缓存命中日志
		return v, nil
	}

	// 缓存未命中，加载数据
	return g.load(key)
}

// load 数据加载方法（当前为本地加载）
// 预留位置，未来可扩展为分布式加载
func (g *Group) load(key string) (value ByteView, err error) {
	return g.getLocally(key) // 目前仅本地加载
}

// getLocally 从本地数据源获取数据并填充缓存
func (g *Group) getLocally(key string) (ByteView, error) {
	// 调用用户提供的数据获取器
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err // 转发数据获取错误
	}

	// 封装为不可变字节视图
	value := ByteView{b: cloneBytes(bytes)}

	// 填充缓存（非阻塞）
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将数据添加到缓存
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value) // 添加到主缓存
}
