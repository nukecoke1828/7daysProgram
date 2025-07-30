package geecache

import pb "github.com/nukecoke1828/7daysProgram/GeeCache/geecache/geecachepb"

// PeerPicker 接口定义了节点选择器的行为
// 在分布式缓存系统中，用于根据键(key)选择对应的远程节点
type PeerPicker interface {
	// PickPeer 根据键选择对应的节点
	// 参数:
	//   key - 要查询的缓存键
	// 返回值:
	//   peer - 找到的远程节点访问器(PeerGetter)
	//   ok   - 是否成功找到节点
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 接口定义了从远程节点获取缓存值的行为
// 用于与缓存集群中的其他节点进行通信
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
