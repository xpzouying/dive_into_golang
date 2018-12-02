package main

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
)

func main() {
	beego.Get("/", func(ctx *context.Context) {
		ctx.Output.Body([]byte("hello world"))
	})

	beego.Post("/alice", func(ctx *context.Context) {
		ctx.Output.Body([]byte("bob"))
	})

	beego.Any("/foo", func(ctx *context.Context) {
		ctx.Output.Body([]byte("bar"))
	})

	beego.Run()
}
