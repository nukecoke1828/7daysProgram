package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash 定义哈希函数类型
type Hash func(data []byte) uint32

// Map 实现一致性哈希的核心数据结构
type Map struct {
	hash     Hash           // 注入的哈希函数
	replicas int            // 每个真实节点对应的虚拟节点数量
	keys     []int          // 排序存储的所有节点哈希值（哈希环）
	hashMap  map[int]string // 节点哈希值到真实节点名称的映射
}

// New 创建一致性哈希Map实例
// replicas: 每个真实节点生成的虚拟节点数量
// fn: 可注入的自定义哈希函数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	// 如果未注入哈希函数，使用默认的CRC32算法
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 添加真实节点到哈希环
// keys: 一个或多个真实节点标识
func (m *Map) Add(keys ...string) {
	// 遍历所有真实节点
	for _, key := range keys {
		// 为每个真实节点创建replicas个虚拟节点
		for i := 0; i < m.replicas; i++ {
			// 生成虚拟节点名称：strconv.Itoa(i) + key
			// 示例：节点"192.168.1.1" -> 虚拟节点["0192.168.1.1", "1192.168.1.1"...]
			virtualNode := strconv.Itoa(i) + key

			// 计算虚拟节点哈希值
			hash := int(m.hash([]byte(virtualNode)))

			// 将虚拟节点哈希值加入环
			m.keys = append(m.keys, hash)

			// 建立虚拟节点哈希值到真实节点的映射
			m.hashMap[hash] = key
		}

		// 注意：这里没有直接将真实节点加入环
		// 真实节点仅通过其虚拟节点间接存在于环上
	}
	// 对哈希环上的所有哈希值进行排序
	sort.Ints(m.keys)
}

// Get 根据键查找对应的真实节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	// 计算键的哈希值
	hash := int(m.hash([]byte(key)))

	// 在排序的哈希环中二分查找第一个 >= 键哈希的节点
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	// 环形处理：当索引超出范围时回到环首
	targetHash := m.keys[idx%len(m.keys)]

	// 通过虚拟节点哈希值找到对应的真实节点
	return m.hashMap[targetHash]
}
