package lru

import "container/list"

// Cache 是一个LRU（最近最少使用）缓存结构。
// 当缓存达到最大容量时，会自动淘汰最久未使用的项目。
type Cache struct {
	maxBytes  int64                         // 缓存的最大容量（以字节为单位），0表示无限制
	nbytes    int64                         // 当前缓存已使用的总字节数（包括键和值）
	ll        *list.List                    // 双向链表，用于实现LRU策略，链表头是最近使用的元素
	cache     map[string]*list.Element      // 哈希表，用于存储键到链表元素的映射
	OnEvicted func(key string, value Value) // 可选的回调函数，在项目被淘汰时调用
}

// entry 是链表节点中存储的数据结构
type entry struct {
	key   string // 缓存的键
	value Value  // 缓存的值
}

// Value 是缓存值必须实现的接口
type Value interface {
	Len() int // 返回值占用的内存大小
}

// New 创建一个新的LRU缓存实例
// maxBytes: 缓存的最大容量（字节），0表示无限制
// onEvicted: 淘汰项目时的回调函数（可为nil）
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Get 从缓存中获取键对应的值
// 返回值：值（如果存在）和布尔值（表示是否命中）
// 如果命中，会将项目移动到链表头部（表示最近使用）
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, exists := c.cache[key]; exists {
		c.ll.MoveToFront(ele)    // 将元素移动到链表头部（最近使用）
		kv := ele.Value.(*entry) // 类型断言获取节点数据
		return kv.value, true
	}
	return nil, false
}

// RemoveOldest 淘汰链表尾部的项目（最久未使用）
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() // 获取链表尾部元素（最久未使用）
	if ele != nil {
		c.ll.Remove(ele)                                       // 从链表中移除
		kv := ele.Value.(*entry)                               // 获取节点数据
		delete(c.cache, kv.key)                                // 从哈希表中删除键
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) // 更新已用内存
		if c.OnEvicted != nil {                                // 如果设置了回调
			c.OnEvicted(kv.key, kv.value) // 执行回调函数
		}
	}
}

// Add 向缓存中添加/更新键值对
// 如果键已存在：更新值并将项目移到链表头部
// 如果键不存在：在链表头部添加新项目，并更新内存计数
// 添加后如果超过最大内存，则循环淘汰最久未使用的项目直到满足容量限制
func (c *Cache) Add(key string, value Value) {
	if ele, exists := c.cache[key]; exists { // 键已存在
		c.ll.MoveToFront(ele)    // 移动到链表头部
		kv := ele.Value.(*entry) // 获取旧值
		// 更新内存：新值大小 - 旧值大小
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value // 更新值
	} else { // 新键
		ele := c.ll.PushFront(&entry{key, value}) // 在链表头部插入新节点
		c.cache[key] = ele                        // 添加到哈希表
		// 增加内存：键长 + 值大小
		c.nbytes += int64(len(key)) + int64(value.Len())
	}

	// 如果设置了最大内存（非0）且当前内存超出，则循环淘汰
	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}
}

// Len 返回缓存中的项目数量
func (c *Cache) Len() int {
	return c.ll.Len()
}
