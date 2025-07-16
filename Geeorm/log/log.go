package log

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
)

const (
	InfoLevel = iota
	ErrorLevel
	Disabled
)

var ( //使用os.Stdout作为输出，更加灵活性能更好，符合依赖倒置原则
	errorLog = log.New(os.Stdout, "\033[31m[ERROR]\033[0m ", log.LstdFlags|log.Lshortfile) // 错误日志记录器（红色前缀)
	infoLog  = log.New(os.Stdout, "\033[34m[INFO]\033[0m ", log.LstdFlags|log.Lshortfile)  // 信息日志记录器（蓝色前缀)
	loggers  = []*log.Logger{errorLog, infoLog}                                            //日志记录器数组
	mu       sync.Mutex
)

var (
	Error  = errorLog.Println
	Errorf = errorLog.Printf
	Info   = infoLog.Println
	Infof  = infoLog.Printf
)

// 只有设置日志级别时才调用
func SetLevel(level int) {
	mu.Lock() // 确保线程安全
	defer mu.Unlock()
	for _, logger := range loggers { // 重置所有日志输出
		logger.SetOutput(os.Stdout)
	}
	if ErrorLevel < level { // 丢弃错误日志，优化性能
		errorLog.SetOutput(ioutil.Discard)
	}
	if InfoLevel < level {
		infoLog.SetOutput(ioutil.Discard)
	}
}
