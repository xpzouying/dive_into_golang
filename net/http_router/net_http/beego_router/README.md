# dive into beego/router

上一次分析了默认的http server以及router的工作方式。

具体参见：

- [Dive into golang: http router](https://github.com/xpzouying/dive_into_golang/tree/master/net/http_router)

从今天开始分析国人创建的一个非常有名的web框架——[beego](https://github.com/astaxie/beego)。

从github主页的hello world开始，

```go
package main

import "github.com/astaxie/beego"

func main(){
    beego.Run()
}
```

Build and run

```bash
go build hello.go
./hello
```

打开浏览器，访问`http://localhost:8080`，得到了

![beego_router_1.jpg](images/beego_router_1.jpg)


从之前对于默认的http router可以推测到beego对于查找不到的路由handler也会进行默认的处理，与go默认的不同是，

默认的只是返回一个404错误和一个字符串，"404 page not found"。

```go
// NotFound replies to the request with an HTTP 404 not found error.
func NotFound(w ResponseWriter, r *Request) { Error(w, "404 page not found", StatusNotFound) }

// NotFoundHandler returns a simple request handler
// that replies to each request with a ``404 page not found'' reply.
func NotFoundHandler() Handler { return HandlerFunc(NotFound) }
```

而beego会返回一个HTML页面。

具体的是如何实现的，我们后面分析代码的时候会追踪到。

首先分析`beego.Run()`，看看beego.Run()在做什么？

> 猜测：
> beego.Run()在底层至少包括下面两步：
> 1. 创建了默认的router，其中包括一个NotFoundHandler来处理404错误；从上一次分析http router可以得知，
> router肯定会满足http中的ServeHTTP interface，这样才能满足对应的Handler规范。
> 2. 启动HTTP service，监听socket地址，当接收到请求的时候，调用对应的handler进行处理；

下面分析源码：

```go
// Run beego application.
// beego.Run() default run on HttpPort
// beego.Run("localhost")
// beego.Run(":8089")
// beego.Run("127.0.0.1:8089")
func Run(params ...string) {

	initBeforeHTTPRun()
    // ...
	BeeApp.Run()
}
```

主要包括两部分：

1. `initBeforeHTTPRun()`

2. `BeeApp.Run()`


先看第1部分，在运行http server之前做了一些行为的处理。深入进去看看都是做了什么？

```go
func initBeforeHTTPRun() {
	//init hooks
	AddAPPStartHook(
		registerMime,
		registerDefaultErrorHandler,
		registerSession,
		registerTemplate,
		registerAdmin,
		registerGzip,
	)

	for _, hk := range hooks {
		if err := hk(); err != nil {
			panic(err)
		}
	}
}
```

做了一大堆的hook，这些hook是什么呢？从后面for循环中可以看出是一个接口规范，在启动前会注册一大堆的参数或者配置，
其中有一个`registerDefaultErrorHandler`，应该就是我们要找的404页面的Handler处理函数。

我们以这个处理函数的hook为例，看看都在http server运行前都在做什么？

```go
// register default error http handlers, 404,401,403,500 and 503.
func registerDefaultErrorHandler() error {
	m := map[string]func(http.ResponseWriter, *http.Request){
		"401": unauthorized,
		"402": paymentRequired,
		"403": forbidden,
		"404": notFound,
		"405": methodNotAllowed,
		"500": internalServerError,
		"501": notImplemented,
		"502": badGateway,
		"503": serviceUnavailable,
		"504": gatewayTimeout,
		"417": invalidxsrf,
		"422": missingxsrf,
	}
	for e, h := range m {
		if _, ok := ErrorMaps[e]; !ok {
			ErrorHandler(e, h)
		}
	}
	return nil
}
```

为4xx、5xx做了预先定义了一大堆的http HandleFunc，我们还是看我们的404错误的处理函数：notFound，

```go
// show 404 not found error.
func notFound(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		404,
		"<br>The page you have requested has flown the coop."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>The page has moved"+
			"<br>The page no longer exists"+
			"<br>You were looking for your puppy and got lost"+
			"<br>You like 404 pages"+
			"</ul>",
	)
}

func responseError(rw http.ResponseWriter, r *http.Request, errCode int, errContent string) {
	t, _ := template.New("beegoerrortemp").Parse(errtpl)
	data := M{
		"Title":        http.StatusText(errCode),
		"BeegoVersion": VERSION,
		"Content":      template.HTML(errContent),
	}
	t.Execute(rw, data)
}
```


首先按照HandleFunc interface的规范，定义了`notFound`的处理函数，这个函数使用template库渲染了html页面。

我们最开始打开的404页面就是由该处理函数进行渲染得到的。

在`registerDefaultErrorHandler`中其他的错误处理函数也是类似，使用http template渲染了不同的html页面。

下面这段for循环代码，是刚才那一堆默认的error handler注册到ErrorMaps上面。如果用户定义了自己的错误处理函数，那么使用用户自己定义的，否则使用beego自带的。

```go
	for e, h := range m {
		if _, ok := ErrorMaps[e]; !ok { // 如果用户没有定义，则使用beego自带的错误处理函数
			ErrorHandler(e, h)
		}
	}
```

`ErrorMaps`是什么呢？

```go
// ErrorMaps holds map of http handlers for each error string.
// there is 10 kinds default error(40x and 50x)
var ErrorMaps = make(map[string]*errorInfo, 10)

// ErrorHandler registers http.HandlerFunc to each http err code string.
// usage:
// 	beego.ErrorHandler("404",NotFound)
//	beego.ErrorHandler("500",InternalServerError)
func ErrorHandler(code string, h http.HandlerFunc) *App {
	ErrorMaps[code] = &errorInfo{
		errorType: errorTypeHandler,
		handler:   h,
		method:    code,
	}
	return BeeApp
}
```

ErrorMaps是一个默认的查找错误的处理函数的表，这也是router mux的一部分，只是它专门用来查找错误的HandleFunc。

ErrorHandler函数在注册完后，返回一个BeeApp，其类型为`*App`，看起来BeeApp就是咱们要找的默认的http server以及handler集合了。

```go
var (
	// BeeApp is an application instance
	BeeApp *App
)

// App defines beego application with a new PatternServeMux.
type App struct {
	Handlers *ControllerRegister
	Server   *http.Server
}
```


再分析其他的Hook都是干什么？大部分暂时跳过，我们主要是要找到运行的原理，暂时不纠结细节处理。

- `registerMime`：注册一大堆的文件与文件在HTTP Header上Content-Type对应关系，比如`".json"`注册为`"application/json"`。

- `registerDefaultErrorHandler`：注册错误的处理函数

- `registerSession`：跳过。

- `registerTemplate`：跳过。

- `registerAdmin`：跳过。

- `registerGzip`：跳过。


接下来看看初始化完后，http service是如何启动并且找到对应的handler的？

重点看BeeApp，App struct中包括两个，

1. `Handlers *ControllerRegister`

2. `Server   *http.Server`


`BeeApp.Run()`启动后，


```go
// MiddleWare function for http.Handler
type MiddleWare func(http.Handler) http.Handler

// Run beego application.
func (app *App) Run(mws ...MiddleWare) {
	addr := BConfig.Listen.HTTPAddr

	if BConfig.Listen.HTTPPort != 0 {
		addr = fmt.Sprintf("%s:%d", BConfig.Listen.HTTPAddr, BConfig.Listen.HTTPPort)
	}

	var (
		err        error
		l          net.Listener
		endRunning = make(chan bool, 1)
	)
    // ...
	if BConfig.Listen.EnableHTTP {
		go func() {
			app.Server.Addr = addr
			logs.Info("http server Running on http://%s", app.Server.Addr)
			if BConfig.Listen.ListenTCP4 {
				ln, err := net.Listen("tcp4", app.Server.Addr)
				// ...
				if err = app.Server.Serve(ln); err != nil {
					// ...
					endRunning <- true
					return
				}
			} // ...
		}()
	}
	<-endRunning
}
```


暂时跳过MiddleWare相关，后面专门分析MiddleWare相关知识点。

我们查看普通的http模式，进入到`if BConfig.Listen.EnableHTTP`分支，并且我们假设仅仅监听tcp ipv4，而不监听ipv6。

在这里面起了一个goroutine运行，然后在整个函数的最后使用endRunning的channel等待goroutine的工作完毕。

goroutine中做了什么事情？

1. 使用net.Listen做了一个tcp v4的监听；

2. 使用app中的Server对这个连接进行处理(Serve)；Serve会从监听的listen接收到进来的请求，然后创建一个新的goroutine去执行。在goroutine中，服务器端会读取request的信息，并且找到对应的srv.Handler去处理请求。

> TODO：
> 不清楚为什么在Run()里面需要起一个goroutine来接收/处理请求。
> 我的感觉是不需要用goroutine，也不需要endRunning channel来阻塞。在Serve(ln)中，已经包括一个for的死循环了，程序不会结束退出。