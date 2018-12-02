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

3. addToRouter中可以看到对于每一种method，都是各自的一个Tree。这个跟http default mux有区别，在default mux中，所有的方法都同时引导到同一个handler func中，在那里面对每一种method进行区分。

```go
func (p *ControllerRegister) addToRouter(method, pattern string, r *ControllerInfo) {
	if !BConfig.RouterCaseSensitive {
		pattern = strings.ToLower(pattern)
	}
	if t, ok := p.routers[method]; ok {
		t.AddRouter(pattern, r)
	} else {
		t := NewTree()
		t.AddRouter(pattern, r)
		p.routers[method] = t
	}
}

// AddRouter call addseg function
func (t *Tree) AddRouter(pattern string, runObject interface{}) {
	t.addseg(splitPath(pattern), runObject, nil, "")
}

// "/" -> []
// "/admin" -> ["admin"]
// "/admin/" -> ["admin"]
// "/admin/users" -> ["admin", "users"]
func splitPath(key string) []string {
	key = strings.Trim(key, "/ ")
	if key == "" {
		return []string{}
	}
	return strings.Split(key, "/")
}
```

在splitPath中可以看到，在注册路径的时候，按照`/`进行了路径分割，然后保存到了一个`[]string`slice中，注册的形式比较特殊，跟http default mux直接把pattern放进去不同，default mux搜索时做了最长路径匹配的搜索，所以在搜索的过程中做了很多处理用来实现最长路径匹配，这也是导致default mux搜索慢的原因，不知道beego的这种注册方式是不是在优化这个搜索过程，后面分析到搜索的时候，详细看看。

```go
// "/"
// "admin" ->
func (t *Tree) addseg(segments []string, route interface{}, wildcards []string, reg string) {
	if len(segments) == 0 {
		if reg != "" {
			t.leaves = append(t.leaves, &leafInfo{runObject: route, wildcards: wildcards, regexps: regexp.MustCompile("^" + reg + "$")})
		} else {
			t.leaves = append(t.leaves, &leafInfo{runObject: route, wildcards: wildcards})
		}
	} else {
		seg := segments[0]
		iswild, params, regexpStr := splitSegment(seg)
		// if it's ? meaning can igone this, so add one more rule for it
		if len(params) > 0 && params[0] == ":" {
			t.addseg(segments[1:], route, wildcards, reg)
			params = params[1:]
		}
		//Rule: /login/*/access match /login/2009/11/access
		//if already has *, and when loop the access, should as a regexpStr
		if !iswild && utils.InSlice(":splat", wildcards) {
			iswild = true
			regexpStr = seg
		}
		//Rule: /user/:id/*
		if seg == "*" && len(wildcards) > 0 && reg == "" {
			regexpStr = "(.+)"
		}
		if iswild {
			if t.wildcard == nil {
				t.wildcard = NewTree()
			}
			if regexpStr != "" {
				if reg == "" {
					rr := ""
					for _, w := range wildcards {
						if w == ":splat" {
							rr = rr + "(.+)/"
						} else {
							rr = rr + "([^/]+)/"
						}
					}
					regexpStr = rr + regexpStr
				} else {
					regexpStr = "/" + regexpStr
				}
			} else if reg != "" {
				if seg == "*.*" {
					regexpStr = "/([^.]+).(.+)"
					params = params[1:]
				} else {
					for range params {
						regexpStr = "/([^/]+)" + regexpStr
					}
				}
			} else {
				if seg == "*.*" {
					params = params[1:]
				}
			}
			t.wildcard.addseg(segments[1:], route, append(wildcards, params...), reg+regexpStr)
		} else {
			var subTree *Tree
			for _, sub := range t.fixrouters {
				if sub.prefix == seg {
					subTree = sub
					break
				}
			}
			if subTree == nil {
				subTree = NewTree()
				subTree.prefix = seg
				t.fixrouters = append(t.fixrouters, subTree)
			}
			subTree.addseg(segments[1:], route, wildcards, reg)
		}
	}
}
```

上面这段代码是如何注册router的详细过程。

1. 从input分析

   1. segments: 为路径按照`/` split分割后的字符串数组
   2. route: 为route，里面包含了http handle func
   3. wildcards: 通配符，nil。如果我们的节点是通配符的话，那么我们会保留我们所有的通配符定义，比如["id" "name"] for the wildcard ":id" and ":name"。
   4. reg: 为空字符串，表示我们不是一个正则表达式

2. 函数里面按照segments的不同，进行了不同的处理。在第一个注册函数中，我们只是对路径`/`进行了注册，所以在这里的segments的长度也就是0，并且我们的没有使用正则表达式。

   我们在该函数里面仅仅是调用了一行语句：

   ```go
   			t.leaves = append(t.leaves, &leafInfo{runObject: route, wildcards: wildcards})
   ```

   我们在Tree的叶子结点上增加了一个叶子结点的信息：`route`，也就是把我们的http handle func的所有信息都传递进去，wildcards为nil，没有通配符。

当该http处理函数注册完成后，启动http server时，我们下面分析查找的过程。

### beego router 搜索过程

调用`BeeApp.Run()`，其实在底层就是调用了http Serve(l)，这其中的过程参考http router的解析，在这里就不再重复。

最后会调用到http/server.go中的下列方法，

```go
func (sh serverHandler) ServeHTTP(rw ResponseWriter, req *Request) {
    handler := sh.srv.Handler
    if handler == nil {
        handler = DefaultServeMux
    }
    if req.RequestURI == "*" && req.Method == "OPTIONS" {
        handler = globalOptionsHandler{}
    }
    handler.ServeHTTP(rw, req)
}
```

在http router中解析过该函数，当时由于sh.srv.Handler为nil，所以使用的是默认的DefaultServeMux。

但是在当前beego中，Handler不再为nil，在之前分析过，当前的Handler为`ControllerRegister`类型，所以其实当运行的过程中，接收到一个请求后，会进入到`ControllerRegister.ServeHTTP`方法。

```go
// Implement http.Handler interface.
func (p *ControllerRegister) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	var (
		runRouter    reflect.Type
		findRouter   bool
		runMethod    string
		methodParams []*param.MethodParam
		routerInfo   *ControllerInfo
		isRunnable   bool
	)
	context := p.pool.Get().(*beecontext.Context)
	context.Reset(rw, r)

	defer p.pool.Put(context)
	if BConfig.RecoverFunc != nil {
		defer BConfig.RecoverFunc(context)
	}

	context.Output.EnableGzip = BConfig.EnableGzip

	if BConfig.RunMode == DEV {
		context.Output.Header("Server", BConfig.ServerName)
	}

	var urlPath = r.URL.Path

	if !BConfig.RouterCaseSensitive {
		urlPath = strings.ToLower(urlPath)
	}

	// filter wrong http method
	if !HTTPMETHOD[r.Method] {
		http.Error(rw, "Method Not Allowed", 405)
		goto Admin
	}

	// filter for static file
	if len(p.filters[BeforeStatic]) > 0 && p.execFilter(context, urlPath, BeforeStatic) {
		goto Admin
	}

	serverStaticRouter(context)

	if context.ResponseWriter.Started {
		findRouter = true
		goto Admin
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// ...
	}

	// session init
	if BConfig.WebConfig.Session.SessionOn {
		// ...
	}
	if len(p.filters[BeforeRouter]) > 0 && p.execFilter(context, urlPath, BeforeRouter) {
		goto Admin
	}
	// User can define RunController and RunMethod in filter
	if context.Input.RunController != nil && context.Input.RunMethod != "" {
		findRouter = true
		runMethod = context.Input.RunMethod
		runRouter = context.Input.RunController
	} else {
		routerInfo, findRouter = p.FindRouter(context)
	}

	//if no matches to url, throw a not found exception
	if !findRouter {
		exception("404", context)
		goto Admin
	}
	if splat := context.Input.Param(":splat"); splat != "" {
		for k, v := range strings.Split(splat, "/") {
			context.Input.SetParam(strconv.Itoa(k), v)
		}
	}

	//execute middleware filters
	if len(p.filters[BeforeExec]) > 0 && p.execFilter(context, urlPath, BeforeExec) {
		goto Admin
	}

	//check policies
	if p.execPolicy(context, urlPath) {
		goto Admin
	}

	if routerInfo != nil {
		//store router pattern into context
		context.Input.SetData("RouterPattern", routerInfo.pattern)
		if routerInfo.routerType == routerTypeRESTFul {
			if _, ok := routerInfo.methods[r.Method]; ok {
				isRunnable = true
				routerInfo.runFunction(context)
			} else {
				exception("405", context)
				goto Admin
			}
		} else if routerInfo.routerType == routerTypeHandler {
			isRunnable = true
			routerInfo.handler.ServeHTTP(rw, r)
		} else {
			runRouter = routerInfo.controllerType
			methodParams = routerInfo.methodParams
			method := r.Method
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodPost {
				method = http.MethodPut
			}
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodDelete {
				method = http.MethodDelete
			}
			if m, ok := routerInfo.methods[method]; ok {
				runMethod = m
			} else if m, ok = routerInfo.methods["*"]; ok {
				runMethod = m
			} else {
				runMethod = method
			}
		}
	}

	// also defined runRouter & runMethod from filter
	if !isRunnable {
		// ...
	}

	//execute middleware filters
	if len(p.filters[AfterExec]) > 0 && p.execFilter(context, urlPath, AfterExec) {
		goto Admin
	}

	if len(p.filters[FinishRouter]) > 0 && p.execFilter(context, urlPath, FinishRouter) {
		goto Admin
	}

Admin:
	//admin module record QPS

	statusCode := context.ResponseWriter.Status
	if statusCode == 0 {
		statusCode = 200
	}

	logAccess(context, &startTime, statusCode)

	timeDur := time.Since(startTime)
	context.ResponseWriter.Elapsed = timeDur
	if BConfig.Listen.EnableAdmin {
		// ... 如果启动admin，跳过
	}

	if BConfig.RunMode == DEV && !BConfig.Log.AccessLogs {
		// ... 如果是开发模式，跳过
	}
	// Call WriteHeader if status code has been set changed
	if context.Output.Status != 0 {
		context.ResponseWriter.WriteHeader(context.Output.Status)
	}
}

// Reset init Context, BeegoInput and BeegoOutput
func (ctx *Context) Reset(rw http.ResponseWriter, r *http.Request) {
    ctx.Request = r
    if ctx.ResponseWriter == nil {
        ctx.ResponseWriter = &Response{}
    }
    ctx.ResponseWriter.reset(rw)
    ctx.Input.Reset(ctx)
    ctx.Output.Reset(ctx)
    ctx._xsrfToken = ""
}
```



1. `context(beecontext.Context)`使用了sync.Pool做cache优化。每次在调用`Reset`时，把请求的`http.ResponseWriter`和`http.Request`传给context。
2. 定义了`RecoverFunc`用来恢复用。这个还比较感兴趣，记下TODO以后分析。
3. method的请求方法如果beego不支持，则会写405，Method Not Allowed错误，不过大部分http method都支持。
4. 搜索对应的router，调用的是`routerInfo, findRouter = p.FindRouter(context)`，后面详细分析。
5. 我们当前的请求为：`curl -X GET http://localhost:8080`，我们是可以找到对应的router。所以我们进入`if routerInfo != nil`分支中分析。我们在注册的时候，使用的是`route.routerType = routerTypeRESTFul`，所以我们我们会得到两个部分：
   1. `isRunnable = true`
   2. `routerInfo.runFunction(context)`：这里就是调用到了我们定义的HandleFunc了，我们只是简单的进行了简单的输出`ctx.Output.Body([]byte("hello world"))`。
6. 由于我们当前的isRunnable是true，所以if分支里面的一大坨代码暂时跳过。后面的代码也暂时跳过。分析之前省略的如何搜索到对应的router。



#### 如何搜索找到对应的router

```go
// FindRouter Find Router info for URL
func (p *ControllerRegister) FindRouter(context *beecontext.Context) (routerInfo *Controller
Info, isFind bool) {
    var urlPath = context.Input.URL()
    if !BConfig.RouterCaseSensitive {
        urlPath = strings.ToLower(urlPath)
    }
    httpMethod := context.Input.Method()
    if t, ok := p.routers[httpMethod]; ok {
        runObject := t.Match(urlPath, context)
        if r, ok := runObject.(*ControllerInfo); ok {
            return r, true
        }
    }
    return
}
```



由于我们把http request的信息都放到了beego.context中的Input了，所以我们从context中找到请求的所有信息。这里我们需要根据request的url和method进行router的搜索匹配。

