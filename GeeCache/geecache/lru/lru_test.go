package lru

import (
	"reflect"
	"testing"
)

// String 类型用于测试，实现了 Value 接口
type String string

// Len 返回字符串的长度（实现 Value 接口）
func (d String) Len() int {
	return len(d)
}

// TestGet 测试缓存的基本功能：添加和获取
func TestGet(t *testing.T) {
	lru := New(int64(0), nil)       // 创建无容量限制的缓存
	lru.Add("key1", String("1234")) // 添加键值对

	// 测试获取存在的键
	if v, ok := lru.Get("key1"); !ok || v.(String) != "1234" {
		t.Fatalf("cache hit key1=1234 failed") // 验证值是否正确
	}

	// 测试获取不存在的键
	if _, ok := lru.Get("key2"); ok {
		t.Fatalf("cache miss key2 failed") // 应返回不存在
	}
}

// TestRemoveoldest 测试LRU淘汰机制
func TestRemoveoldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	cap := len(k1 + k2 + v1 + v2) // 计算仅能容纳前两个键值对的容量

	lru := New(int64(cap), nil) // 创建有容量限制的缓存
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3)) // 添加第三个键值对会触发淘汰

	// 验证第一个键是否被淘汰
	if _, ok := lru.Get(k1); ok || lru.Len() != 2 {
		t.Fatalf("Remove oldest key1 failed") // 期望: key1被移除且缓存只剩2个元素
	}
}

// TestOnEvicted 测试淘汰回调函数
func TestOnEvicted(t *testing.T) {
	keys := make([]string, 0) // 记录被淘汰的键
	callback := func(key string, value Value) {
		keys = append(keys, key) // 回调时记录被淘汰的键
	}

	// 创建容量为10字节的缓存（仅能容纳约两个键值对）
	lru := New(int64(10), callback)

	// 添加键值对（每个键值对大小：key1=7字节，后续每个约2-3字节）
	lru.Add("key1", String("123456")) // 10字节（key1=4 + value=6）
	lru.Add("k2", String("k2"))       // 4字节 → 触发淘汰
	lru.Add("k3", String("k3"))       // 4字节
	lru.Add("k4", String("k4"))       // 4字节 → 再次触发淘汰

	expect := []string{"key1", "k2"} // 预期淘汰顺序：最先添加的key1，然后是k2

	// 验证回调函数是否正确记录了被淘汰的键
	if !reflect.DeepEqual(keys, expect) {
		t.Fatalf("Call OnEvicted failed, expect keys %v but got %v", expect, keys)
	}
}
