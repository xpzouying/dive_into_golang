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

在`app.go`的init()方法中，默认的BeeApp方法做了初始化NewApp()。

在`NewApp()`中，调用了`NewControllerRegister()`初始化了Handlers。

在`Run(mws ...MiddleWare)`中进行了赋值，`app.Server.Handler = app.Handlers`，把App中的`*ControllerRegister`赋值给了Server.Handler。

从这里也可以猜到`*ControllerRegister`类型实现了Handler的接口，即实现了`ServeHTTP(ResponseWriter, *Request)`函数。

追踪到`NewControllerRegister()`中，

### 分析NewControllerRegister()方法

```go
// ControllerRegister containers registered router rules, controller handlers and filters.
type ControllerRegister struct {
	routers      map[string]*Tree
	enablePolicy bool
	policies     map[string]*Tree
	enableFilter bool
	filters      [FinishRouter + 1][]*FilterRouter
	pool         sync.Pool
}


// NewControllerRegister returns a new ControllerRegister.
func NewControllerRegister() *ControllerRegister {
	return &ControllerRegister{
		routers:  make(map[string]*Tree),
		policies: make(map[string]*Tree),
		pool: sync.Pool{
			New: func() interface{} {
				return beecontext.NewContext()
			},
		},
	}
}
```

我们先考虑普通的情况，再考虑通配符（例如:id）的情况。

NewControllerRegister()就是创建了ControllerRegister，包含了注册的路由规则、http处理函数、以及filters（这个是做什么的？后面分析，暂时跳过）。

其中如何搜索router handler是使用Tree来实现，这里与go http中默认的mux使用map搜索不同。

### 分析注册一个handler

```go
	beego.Get("/", func(ctx *context.Context) {
		ctx.Output.Body([]byte("hello world"))
	})
```

当我们把一个handler注册给一个路径的时候，都做了什么操作？

需要注意的是这里的context是beego/context，与go中的不同。

```go
// Get used to register router for Get method
// usage:
//    beego.Get("/", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Get(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Get(rootpath, f)
	return BeeApp
}
```

beego中自定义了http handler func类型，为`FilterFunc`，定义如下，

```go
// FilterFunc defines a filter function which is invoked before the controller handler is executed.
type FilterFunc func(*context.Context)
```

我们调用beego.Get方法，也即是调用了BeeApp.Handlers.Get，

```go
// Get add get method
// usage:
//    Get("/", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Get(pattern string, f FilterFunc) {
	p.AddMethod("get", pattern, f)
}

// AddMethod add http method router
// usage:
//    AddMethod("get","/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) AddMethod(method, pattern string, f FilterFunc) {
	method = strings.ToUpper(method)
	if method != "*" && !HTTPMETHOD[method] {
		panic("not support http method: " + method)
	}
	route := &ControllerInfo{}
	route.pattern = pattern
	route.routerType = routerTypeRESTFul
	route.runFunction = f
	methods := make(map[string]string)
	if method == "*" {
		for val := range HTTPMETHOD {
			methods[val] = val
		}
	} else {
		methods[method] = method
	}
	route.methods = methods
	for k := range methods {
		if k == "*" {
			for m := range HTTPMETHOD {
				p.addToRouter(m, pattern, route)
			}
		} else {
			p.addToRouter(k, pattern, route)
		}
	}
}
```

当我们使用Get()方法，其实就是定义了处理"get method"行为的处理。

如果我们调用Any()方法，就是定义了处理所有method行为，包括：get、post、put、delete等等。

在AddMethod的方法中，做了下列操作，

1. 新建一个route，类型为`ControllerInfo`，初始化该值

2. 注册新建的route，`p.addToRouter(k, pattern, route)`，在当前行为下，其实就是：`p.addToRouter("get", "/", route)`

