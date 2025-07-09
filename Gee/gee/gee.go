package gee

import (
	"log"
	"net/http"
	"strings"
)

// HandlerFunc定义了处理请求的函数类型
type HandlerFunc func(*Context)

// Engine实现了ServeHTTP接口
type Engine struct {
	router       *router
	*RouterGroup                // 顶级路由组
	groups       []*RouterGroup // 路由组列表
}

type RouterGroup struct {
	prefix      string
	middlewares []HandlerFunc // 中间件列表
	parent      *RouterGroup  // 支持嵌套路由
	engine      *Engine       // 所有路由共享一个Engine实例
}

// New创建Engine实例
func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func (e *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	e.router.addRoute(method, pattern, handler)
}

// GET方法注册路由
func (e *Engine) GET(pattern string, handler HandlerFunc) {
	e.addRoute("GET", pattern, handler)
}

// POST方法注册路由
func (e *Engine) POST(pattern string, handler HandlerFunc) {
	e.addRoute("POST", pattern, handler)
}

// Run启动HTTP服务
func (e *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, e)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var middlewares []HandlerFunc
	for _, group := range e.groups { // 遍历路由组列表
		if strings.HasPrefix(r.URL.Path, group.prefix) { // 匹配路由组前缀
			middlewares = append(middlewares, group.middlewares...) // 合并中间件
		}
	}
	c := newContext(w, r)
	c.handlers = middlewares //将中间件链存入Context
	e.router.handle(c)
}

func (g *RouterGroup) Group(prefix string) *RouterGroup {
	engine := g.engine
	newGroup := &RouterGroup{
		prefix: g.prefix + prefix, // 组合路径
		parent: g,                 // 父路由组
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup) // 加入路由组列表
	return newGroup
}

func (g *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := g.prefix + comp // 组合路径
	log.Printf("Router %4s - %s", method, pattern)
	g.engine.router.addRoute(method, pattern, handler)
}

func (g *RouterGroup) GET(pattern string, handler HandlerFunc) {
	g.addRoute("GET", pattern, handler)
}

func (g *RouterGroup) POST(pattern string, handler HandlerFunc) {
	g.addRoute("POST", pattern, handler)
}

func (g *RouterGroup) Use(middlewares ...HandlerFunc) { // 注册中间件
	g.middlewares = append(g.middlewares, middlewares...)
}
