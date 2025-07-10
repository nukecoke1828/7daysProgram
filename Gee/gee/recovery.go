package gee

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
)

func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil { // 捕获panic
				message := fmt.Sprintf("%s", err)
				log.Printf("%s\n\n", trace(message)) // 获取堆栈信息
				c.Fail(http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		c.Next()
	}
}

// trace 获取堆栈信息
func trace(message string) string {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])
	var str strings.Builder
	str.WriteString(message + "\nTraceback:")
	for _, pc := range pcs[:n] {
		fn := runtime.FuncForPC(pc)
		file, line := fn.FileLine(pc)
		str.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return str.String()
}
