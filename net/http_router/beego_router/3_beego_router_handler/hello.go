package main

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
)

func main() {
	beego.Get("/", func(ctx *context.Context) {
		ctx.Output.Body([]byte("hello world"))
	})

	beego.Post("/hello", func(ctx *context.Context) {
		ctx.Output.Body([]byte("hello zouying"))
	})

	beego.Post("/say/hi", func(ctx *context.Context) {
		ctx.Output.Body([]byte("hi zouying"))
	})

	beego.Any("/say/bye", func(ctx *context.Context) {
		ctx.Output.Body([]byte("bye zouying"))
	})

	beego.Run()
}
