package main

import (
	"net/http"

	"github.com/nukecoke1828/7daysProgram/Gee/gee"
)

func main() {
	r := gee.New()
	r.GET("/", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
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

<<<<<<< HEAD
	r.GET("/hello/:name", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "Hello %s, you're at %s\n", ctx.Param("name"), ctx.Path)
	})

	r.GET("/assets/*filepath", func(ctx *gee.Context) {
		ctx.JSON(http.StatusOK, gee.H{"filepath": ctx.Param("filepath")})
	})

	r.Run(":9999")
}
=======
	r.Run(":9999")
}
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a
