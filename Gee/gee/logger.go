package gee

import (
	"log"
	"time"
)

func Logger() HandlerFunc {
	return func(ctx *Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		method := ctx.Request.Method

		ctx.Next()

		log.Printf("[%s] %s | Status: %d | Time: %v",
			method,
			path,
			ctx.StatusCode,
			time.Since(start))
	}
}
