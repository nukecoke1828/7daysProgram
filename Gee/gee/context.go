package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type H map[string]interface{}

type Context struct {
	//原始对象
	Writer  http.ResponseWriter
	Request *http.Request
	//请求信息
	Path   string
	Method string
	Params map[string]string
	//响应信息
	StatusCode int
	//中间件
	handlers []HandlerFunc
	index    int
	//Engine指针
	engine *Engine
}

func newContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Writer:  w,
		Request: r,
		Path:    r.URL.Path,
		Method:  r.Method,
		index:   -1,
	}
}

func (c *Context) PostForm(key string) string { //获取post表单数据
	return c.Request.FormValue(key)
}

func (c *Context) Query(key string) string { //获取url参数
	return c.Request.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) String(code int, format string, values ...interface{}) {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	c.Status(code)
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) HTML(code int, name string, data interface{}) {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Status(code)
	if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.Fail(http.StatusInternalServerError, err.Error())
	}
}

func (c *Context) Param(key string) string {
	value := c.Params[key]
	return value
}

func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		c.handlers[c.index](c)
	}
}

func (c *Context) Fail(code int, err string) {
	c.index = len(c.handlers) // 跳过后续中间件
	c.JSON(code, H{"message": err})
}
