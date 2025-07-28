package geecache

// ByteView 表示一个不可变的字节序列视图
// 用于封装缓存值，提供只读访问接口
type ByteView struct {
	b []byte // 底层字节切片（不可变）
}

// Len 返回字节视图的长度（实现 Value 接口）
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回底层字节切片的副本
// 避免外部修改原始数据（防御性复制）
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b) // 通过克隆确保原始数据不变
}

// String 将字节视图转换为字符串（只读）
func (v ByteView) String() string {
	return string(v.b) // 转换为字符串（高效，无额外分配）
}

// cloneBytes 创建字节切片的深拷贝
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b) // 复制数据到新切片
	return c
}
