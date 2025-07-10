package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/nukecoke1828/7daysProgram/Gee/gee"
)

type student struct {
	Name string
	Age  int8
}

func onlyForV2() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		t := time.Now()
		ctx.Fail(http.StatusInternalServerError, "Internal Server Error")
		log.Printf("[%d] %s in %v for group v2", ctx.StatusCode, ctx.Request.RequestURI, time.Since(t))
	}
}

func FormatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}

func main() {
	r := gee.Default()
	r.Use(gee.Logger()) //全局中间件
	r.SetFuncMap(template.FuncMap{
		"FormatAsDate": FormatAsDate,
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./static")
	stu1 := &student{Name: "nukecoke", Age: 19}
	stu2 := &student{Name: "nukecola", Age: 20}

	r.GET("/", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "css.tmpl", nil)
	})

	r.GET("/students", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "arr.tmpl", gee.H{
			"title":  "gee",
			"stuArr": []*student{stu1, stu2},
		})
	})

	r.GET("/data", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "custom_func.tmpl", gee.H{
			"title": "custom_func",
			"now":   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	})

	r.GET("/hello", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "Hello %s, you're at %s\n", ctx.Query("name"), ctx.Path)
	})

	r.POST("/login", func(ctx *gee.Context) {
		ctx.JSON(http.StatusOK, gee.H{
			"username": ctx.PostForm("username"),
			"password": ctx.PostForm("password"),
		})
	})

	r.GET("/hello/:name", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "Hello %s, you're at %s\n", ctx.Param("name"), ctx.Path)
	})

	r.GET("/assets/*filepath", func(ctx *gee.Context) {
		ctx.JSON(http.StatusOK, gee.H{"filepath": ctx.Param("filepath")})
	})

	v1 := r.Group("/v1")
	{
		v1.GET("/hello", func(ctx *gee.Context) {
			ctx.String(http.StatusOK, "Hello %s, you're at %s\n", ctx.Query("name"), ctx.Path)
		})
	}
	v2 := r.Group("/v2")
	v2.Use(onlyForV2()) //局部中间件
	{
		v2.GET("/hello/:name", func(ctx *gee.Context) {
			ctx.String(http.StatusOK, "Hello %s, you're at %s\n", ctx.Query("name"), ctx.Path)
		})
		v2.POST("/login", func(ctx *gee.Context) {
			ctx.JSON(http.StatusOK, gee.H{
				"username": ctx.PostForm("username"),
				"password": ctx.PostForm("password"),
			})
		})
	}

	r.GET("/panic", func(ctx *gee.Context) {
		names := []string{"nukecoke"}
		ctx.String(http.StatusOK, names[114514])
	})

	r.Run(":9999")
}
