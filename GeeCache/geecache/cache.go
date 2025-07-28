package geecache

import (
	"sync"

	"github.com/nukecoke1828/7daysProgram/GeeCache/geecache/lru"
)

// cache 是geecache的并发安全缓存封装
// 封装了lru缓存并提供并发安全访问
type cache struct {
	mu         sync.RWMutex // 读写锁，保证并发安全
	lru        *lru.Cache   // 实际的LRU缓存实例
	cacheBytes int64        // 缓存的最大容量（字节）
}

// add 向缓存中添加键值对
// 线程安全：使用互斥锁保护
// 延迟初始化：首次添加时创建LRU缓存
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()         // 获取写锁
	defer c.mu.Unlock() // 确保释放锁

	// 延迟初始化：如果LRU缓存未创建则创建
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}

	c.lru.Add(key, value) // 添加键值对到LRU缓存
}

// get 从缓存中获取值
// 线程安全：使用互斥锁保护
// 返回值：值（如果存在）和布尔值表示是否命中
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()         // 获取写锁（因为lru.Get会修改链表）
	defer c.mu.Unlock() // 确保释放锁

	// 如果LRU缓存未初始化，直接返回未命中
	if c.lru == nil {
		return
	}

	// 从LRU缓存中获取值
	if v, ok := c.lru.Get(key); ok {
		// 类型断言确保返回的是ByteView类型
		return v.(ByteView), ok
	}

	return // 未命中
}
