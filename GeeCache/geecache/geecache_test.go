package geecache

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

// 模拟数据库
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// TestGetter 测试Getter接口实现
func TestGetter(t *testing.T) {
	// 创建GetterFunc实例（函数适配器）
	var f Getter = GetterFunc(func(key string) ([]byte, error) {
		return []byte(key), nil // 简单返回键的字节切片
	})

	expect := []byte("key")

	// 调用Get方法并验证结果
	if v, _ := f.Get("key"); !reflect.DeepEqual(v, expect) {
		t.Errorf("GetterFunc callback failed: expected %v, got %v", expect, v)
	}
}

// TestGet 测试缓存组的核心功能
func TestGet(t *testing.T) {
	// 记录每个键的加载次数（模拟缓存穿透保护）
	loadCounts := make(map[string]int, len(db))

	// 创建缓存组
	gee := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key) // 模拟慢数据库查询

			// 从"数据库"获取数据
			if v, ok := db[key]; ok {
				// 初始化并增加加载计数
				if _, ok := loadCounts[key]; !ok {
					loadCounts[key] = 0
				}
				loadCounts[key]++
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	// 测试1: 首次获取（应加载数据）
	for k, v := range db {
		// 验证获取结果
		if view, err := gee.Get(k); err != nil || view.String() != v {
			t.Fatalf("Failed to get value of %s: expected %s, got error %v", k, v, err)
		}
	}

	// 测试2: 二次获取（应命中缓存）
	for k := range db {
		if _, err := gee.Get(k); err != nil {
			t.Fatalf("Cache miss for %s when should hit", k)
		}
		// 验证加载次数（应为1）
		if loadCounts[k] > 1 {
			t.Fatalf("Cache failed for %s: loaded %d times, expected 1", k, loadCounts[k])
		}
	}

	// 测试3: 获取不存在的数据
	if view, err := gee.Get("unknown"); err == nil {
		t.Fatalf("Expected error for unknown key, got value: %s", view)
	}
}
