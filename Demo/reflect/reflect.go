// 该示例利用 reflect 包在运行时遍历并打印 *sync.WaitGroup 的所有方法签名。
package main

import (
	"log"
	"reflect"
	"strings"
	"sync"
)

func main() {
	var wg sync.WaitGroup      // 声明一个 WaitGroup 实例，但只用来获取类型信息
	typ := reflect.TypeOf(&wg) // 获取 *sync.WaitGroup 的反射类型对象

	// 遍历 *sync.WaitGroup 的所有导出方法
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i) // 第 i 个方法的描述

		// 构造参数名切片（实际使用时仅占位，名字为空字符串）
		argv := make([]string, 0, method.Type.NumIn())
		// 从 1 开始跳过方法接收者（下标 0）
		for j := 1; j < method.Type.NumIn(); j++ {
			argv = append(argv, method.Type.In(j).Name())
		}

		// 构造返回值名切片
		returns := make([]string, 0, method.Type.NumOut())
		for j := 0; j < method.Type.NumOut(); j++ {
			returns = append(returns, method.Type.Out(j).Name())
		}

		// 打印方法签名
		// 例如：func (w *WaitGroup) Add(delta int)
		log.Printf("func (w *%s) %s(%s) %s",
			typ.Elem().Name(),           // WaitGroup
			method.Name,                 // 方法名
			strings.Join(argv, ", "),    // 参数列表
			strings.Join(returns, ", ")) // 返回值列表（若空则省略）
	}
}
