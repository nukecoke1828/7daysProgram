package singleflight

import "sync"

// call 代表一个正在进行中或已完成的函数调用
type call struct {
	wg  sync.WaitGroup // 用于阻塞等待的同步原语
	val interface{}    // 函数调用返回的结果值
	err error          // 函数调用返回的错误
}

// Group 管理不同键(key)的函数调用
type Group struct {
	mu sync.Mutex       // 保护m的互斥锁
	m  map[string]*call // 存储键到调用的映射
}

// Do 确保对于给定键的函数调用只执行一次
// 参数:
//   key - 调用的唯一标识符
//   fn - 实际执行的函数，返回值和错误
// 返回值:
//   interface{} - 函数调用的结果
//   error - 函数调用的错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()

	// 延迟初始化map
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	// 如果该键的调用已存在
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()       // 解锁让其他请求可以进入
		c.wg.Wait()         // 等待该调用完成
		return c.val, c.err // 返回共享的结果
	}

	// 创建新的调用
	c := new(call)
	c.wg.Add(1)   // 添加等待计数器
	g.m[key] = c  // 注册到映射表
	g.mu.Unlock() // 解锁（注意：此时其他相同key的请求会进入等待）

	// 执行实际函数（只有第一个请求执行）
	c.val, c.err = fn()
	c.wg.Done() // 通知所有等待者调用完成

	// 清理调用记录
	g.mu.Lock()
	delete(g.m, key) // 从映射表中删除
	g.mu.Unlock()

	return c.val, c.err
}
