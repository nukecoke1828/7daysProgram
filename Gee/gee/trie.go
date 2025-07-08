package gee

import "strings"

type node struct {
	pattern  string  // 完整路由模式（仅叶子节点设置）
	part     string  //路由分片，如 :id
	children []*node //子节点列表
	isWild   bool    //是否为通配符节点（含:或*）
}

// 查找首个匹配成功的节点,用于插入
func (n *node) matchChild(part string) *node {
	for _, child := range n.children { //遍历子节点
		if child.part == part || child.isWild {
			return child
		}
	}
	return nil
}

// 查找所有匹配成功的节点，用于查找
func (n *node) matchChildren(part string) []*node {
	nodes := make([]*node, 0)
	for _, child := range n.children {
		if child.part == part || child.isWild {
			nodes = append(nodes, child)
		}
	}
	return nodes
}

func (n *node) insert(pattern string, parts []string, height int) {
	if len(parts) == height { //到达叶子节点
		n.pattern = pattern
		return
	}
	part := parts[height]       //获取当前分片
	child := n.matchChild(part) //查找匹配的子节点
	if child == nil {           //没有匹配的子节点，创建新节点
		child = &node{
			part:   part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height+1) //递归插入子节点
}

func (n *node) search(parts []string, height int) *node {
	if len(parts) == height || strings.HasPrefix(n.part, "*") { //到达叶子节点或通配符节点
		if n.pattern == "" { //非叶子节点，且没有匹配的路由
			return nil
		}
		return n
	}
	part := parts[height]
	children := n.matchChildren(part) //查找匹配的子节点列表
	for _, child := range children {
		result := child.search(parts, height+1) //递归查找
		if result != nil {
			return result //找到匹配的节点，返回
		}
	}
	return nil
}
