# beego router handler

前一章分析了beego app是如何启动的，并且分析出来当我们使用默认的beego app启动以后，在没有添加任何的404 handler的情况下，beego是如何为我们添加一个默认的404错误处理页面。


这一章我们分析当我们添加对应的Handler时，http request进来的时候，是如何引导到我们对应的Handler中。

参考beego的文档：https://beego.me/docs/mvc/controller/router.md

我们创建一些路由，

```go
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
```

> 猜测一下，
> 在之前默认的http router学习过程中，我们可以看到由于在启动ListenAndServe时，第二个参数为nil，
> 也即是Server中的Handler为nil，所以导致在最后查找可用的Handler的时候，用的是系统自带的Default Mux，
> 所以我们如果不想用系统默认的mux，那么就需要把Server中的Handler配置上对应的对象。
> 我们接下来分析看beego是如何做的。

我们先把Handler interface的定义放出来，

```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

如果我们想要替换默认的mux的话，那么我们就需要定义一个东西，并且实现了`ServeHTTP(ResponseWriter, *Request)`方法。

App的定义是下列，

```go
var (
	// BeeApp is an application instance
	BeeApp *App
)

func init() {
	// create beego application
	BeeApp = NewApp()
}

// App defines beego application with a new PatternServeMux.
type App struct {
	Handlers *ControllerRegister
	Server   *http.Server
}

// NewApp returns a new beego application.
func NewApp() *App {
	cr := NewControllerRegister()
	app := &App{Handlers: cr, Server: &http.Server{}}
	return app
}
```

我们可以看到App的定义，里面有个Handlers成员，可以猜到Handlers应该和`Server *http.Server`中的Handler可能有关系。

首先看Handlers，在`NewApp()`中，调用了`NewControllerRegister()`出来的。

在`Run(mws ...MiddleWare)`中进行了赋值，`app.Server.Handler = app.Handlers`，把App中的`*ControllerRegister`赋值给了Server.Handler。

从这里也可以猜到`*ControllerRegister`类型实现了Handler的接口，即实现了`ServeHTTP(ResponseWriter, *Request)`函数。

追踪到`NewControllerRegister()`中，

