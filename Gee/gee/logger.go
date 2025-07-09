package gee

import (
	"time"

	"github.com/emicklei/go-restful/v3/log"
)

func Logger() HandlerFunc {
	return func(ctx *Context) {
		t := time.Now() //开始时间
		ctx.Next()      //执行下一个中间件或路由
		log.Printf("[%d] %s in %v", ctx.StatusCode, ctx.Request.RequestURI, time.Since(t))
	}
}
