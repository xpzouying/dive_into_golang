# golang默认的http router


# Day 1

我们从Golang的http router开始学习，学习下面几点：

1. 分析什么是router

2. 解析Go内部是如何实现router

3. 分析为什么Go内部已经有了router后还会出现各种个样的router，他们提供了哪些Go内部router不能完成的功能

4. 我们如何实现一个自定义的router

5. 我们如何在自定义的router上面完成我们想要的各种功能：高效率、Middleware、异常捕获及处理、如何简单的完成JSON类型的返回等等


## DEMO

首先看golang默认的http router是如何工作的。

看一个简单的例子，

```golang
package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
)

func fooHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
}

func main() {
	http.HandleFunc("/foo", fooHandler)

	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

运行：

```bash
go run main.go
```

调用：

```bash
curl http://localhost:8080/foo

# Hello, "/foo"

curl http://localhost:8080/bar

# Hello, "/bar"

curl http://localhost:8080/

# 404 page not found

```

## 现象

1. 注册了两个函数用于处理请求

    - `/foo`：通过`fooHandler`进行处理

    - `/bar`：通过`func(w http.ResponseWriter, r *http.Request)`进行处理

2. HTTP service启动并监听`:8080`端口，等待客户端请求

3. 客户端使用`curl`发起HTTP GET请求时，Golang根据调用的接口路径调用了不同的函数进行处理


## 原理

至关重要的是：如何把客户端的请求与服务器中HTTP处理函数关联起来，这也是路由器的工作。


### 解析

查看`http.ListenAndServe(":8080", nil)`函数的内部，

方法`http.ListenAndServe`原型为：

```go
// ListenAndServe listens on the TCP network address addr and then calls
// Serve with handler to handle requests on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// The handler is typically nil, in which case the DefaultServeMux is used.
//
// ListenAndServe always returns a non-nil error.
func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}
```

`ListenAndServe`从注释上可以看到主要做了两个工作：

1. 监听(listen on)TCP的地址

2. 调用**`handler`**的`Serve`来处理连接进来的请求

如果`handler`，即第2个入参，为`nil`，则使用`DefaultServeMux`。


那么，DefaultServeMux又是什么呢？

```go
// DefaultServeMux is the default ServeMux used by Serve.
var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux
```

从代码上可以看到是`ServeMux`类型的对象，

这个类型的定义及重要解释为，

```go
// ServeMux is an HTTP request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that
// most closely matches the URL.
// ...
type ServeMux struct {
	mu    sync.RWMutex
	m     map[string]muxEntry
	hosts bool // whether any patterns contain hostnames
}

```

ServeMux是一个HTTP的multiplexer，翻译过来就是：数据选择器或者叫多路复用器。

它的作用就是当来了一个HTTP请求后，根据请求的URL匹配服务器端已经注册好的各种处理函数，找到一个最合适的进行处理。

剩下的注释就是解释Golang自带的默认路由器是如何匹配各种情况的，暂时先跳过。



# Day 2

## 如何绑定HandleFunc

http注册一个http处理函数的过程为：

```go
http.HandleFunc("/foo", fooHandler)
```

查看源码为，

```go
// HandleFunc registers the handler function for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}
```

HandleFunc函数的作用就是把一个处理函数注册按照特定的模式注册给默认的DefaultServeMux。

处理函数（HandlerFunc）的定义为：

```go
handler func(ResponseWriter, *Request)
```

在函数里面展示了如何把一个制定的HandlerFunc绑定到DefaultServeMux上。

```go
// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
    // ...
	mux.Handle(pattern, HandlerFunc(handler))
}
```

继续进入mux.Handle分析，

```go
// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
    // ...
	mux.m[pattern] = muxEntry{h: handler, pattern: pattern}

	if pattern[0] != '/' {
		mux.hosts = true
	}
}
```


Handle的作用就是把一种模式注册给http handler。

其实就是把pattern对应的路径规则保存在ServeMux的map表（map[string]muxEntry）中，其中muxEntry的定义如下，

```go
type muxEntry struct {
	h       Handler
	pattern string
}
```

> TODO(zouying)：
> - 为什么map中保存了pattern，这边里面还需要再保存一次？
> 猜测的是估计按照某种规则匹配到对应的http handler以后，还需要再按照某种规则处理一下。

从这里我们了解`如何注册一个http handler到对应的pattern`了，

下一步我们需要揭开的谜题就是：当来了一个请求后，找到对应的http handler的过程。

以default_router/main.go为例，

发起http request后，

`curl -X GET http://localhost:8080/foo`，

如何找到对应的`fooHandler`。


## 思路分析

当client端按照上面的例子发出一个http request的时候，

我们在服务器端需要建立一个http server，这个http server应该绑定socket地址为`localhost:8080`上，协议为http。

那么在go里面，如何建立这样的http server呢？

通过内部的`net/http`包中提供的方法`http.ListenAndServe(":8080", nil)`即可进行监听提供服务。


### http.ListenAndServe方法

```go
func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}
```

前面已经分析过该函数，这个函数就是创建一个Server，监听（listen）入参地址addr，等待连接。

当接收到client端的请求连接时，按照配置的Mux进行处理。其中如果入参Handler为空时，那么使用默认的DefaultServeMux。

在demo中，addr为":8080"，Handler为nil，也意味着我们使用了默认的DefaultServeMux。


接着分析里面的`Server.ListenAndServe()`函数，

首先是分析Server这个结构体，

```go
// A Server defines parameters for running an HTTP server.
// The zero value for Server is a valid configuration.
type Server struct {
	Addr    string  // TCP address to listen on, ":http" if empty
	Handler Handler // handler to invoke, http.DefaultServeMux if nil

	// ...
}
```

Server这个结构体就是定义了HTTP Server的一系列参数，比如监听的地址Addr、注册的处理函数Handler、各种超时参数等等。

其中的Handler是一个重点。

```go
// A Handler responds to an HTTP request.
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

Handler是一个interface，约定了处理HTTP request的接口。

也就是说，如果我们希望定一个函数来响应或者处理接收到的http request请求，那么我们就要实现Handler这个接口约定的`ServeHTTP(ResponseWriter, *Request)`，

其中ResponseWriter是我们相应给client端的headers和data，

其中Request是我们接收到的请求的各种数据。


我们在demo中Server的定义为：`Server{Addr: addr, Handler: handler}`，其他的都是默认值。


我们调用Server.ListenAndServe()进行监听及处理request，所以接着往下追，查看该函数的定义。

```go
// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.
// ...
func (srv *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", addr)
	// ...
	return srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}
```

该函数主要做了如下两步操作，

1. `ln, err := net.Listen("tcp", addr)`：使用tcp在地址addr上进行了监听。其中"tcp"表示即监听了ipv4，也监听了ipv6，如果只需要监听ipv4的话，则使用tcp4。addr为main()中的":8080"。

ln是一个interface。

```go
// A Listener is a generic network listener for stream-oriented protocols.
//
// Multiple goroutines may invoke methods on a Listener simultaneously.
type Listener interface {
	// Accept waits for and returns the next connection to the listener.
	Accept() (Conn, error)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error

	// Addr returns the listener's network address.
	Addr() Addr
}
```

2. `srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})`：首先是将`ln`由`Listener`断言为`TCPListener`，具体作用现在还不确定，但这可以理解，因为我们是使用TCP进行监听。另外使用`tcpKeepAliveListener`封装了一下，具体作用先不深入调查，从命名看猜测可能是加入了一些Keep-Alive相关的参数设置。


下面重点重点分析`svr.Server()`函数的实现。


```go
// Serve accepts incoming connections on the Listener l, creating a
// new service goroutine for each. The service goroutines read requests and
// then call srv.Handler to reply to them.
// ...
func (srv *Server) Serve(l net.Listener) error {
	// ...
	for {
		rw, e := l.Accept()
		// ...
		c := srv.newConn(rw)
		c.setState(c.rwc, StateNew) // before Serve can return
		go c.serve(ctx)
	}
}
```

Serve的工作就是从监听的Listener得到一个连接，然后创建一个goroutine来处理这个请求。该goroutine会读取这个request，然后调用Server中的Handler对进来的（incoming）请求进行处理。

其中Listener在我们的示例中就是tcp在":8080" socket地址上的监听。


分析上面源码，删除非重点代码，得到精华部分。

在一个for循环中，使用Accept()等待一个连接Conn。其中Conn为interface，是一个通用的数据流的网络连接interface。我们可以从Conn进行Read和Write。

```go
// Conn is a generic stream-oriented network connection.
type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	// ...
}
```

Serve中将监听到的Conn进一步使用conn封装了一个新的连接，该连接是一个server side的HTTP连接。该server端的连接应该很重要，做一个TODO，后面详细分析为什么需要建立这个server端的连接。

当建立这个server端的连接后，开启一个goroutine对这个连接进行`serve()`操作。


```go
// Serve a new connection.
func (c *conn) serve(ctx context.Context) {
	c.remoteAddr = c.rwc.RemoteAddr().String()
	ctx = context.WithValue(ctx, LocalAddrContextKey, c.rwc.LocalAddr())
	defer func() {
		// 异常panic处理
	}()

	// ... tls相关的处理

	// HTTP/1.x from here on.

	ctx, cancelCtx := context.WithCancel(ctx)
	c.cancelCtx = cancelCtx
	defer cancelCtx()

	c.r = &connReader{conn: c}
	c.bufr = newBufioReader(c.r)
	c.bufw = newBufioWriterSize(checkConnErrorWriter{c}, 4<<10)

	for {
		w, err := c.readRequest(ctx)
		// ...

		// HTTP cannot have multiple simultaneous active requests.[*]
		// Until the server replies to this request, it can't read another,
		// so we might as well run the handler in this goroutine.
		// [*] Not strictly true: HTTP pipelining. We could let them all process
		// in parallel even if their responses need to be serialized.
		// But we're not going to implement HTTP pipelining because it
		// was never deployed in the wild and the answer is HTTP/2.
		serverHandler{c.server}.ServeHTTP(w, w.req)
		// ...
	}
}
```


首先conn对于对于`c.r`进行封装成`connReader`，这个就是一个io.Reader的一个封装实现。`connReader`在Read的时候，做了一些特殊的处理。这里暂时跳过。

c.bufr就是一个带缓存的Reader，下面其实就是用了bufio.Reader来封装。另外这里的缓存使用了sync.Pool来保存cache，这里可以看到底层的源码使用了很多技巧。sync.Pool我在百度工作的时候，因为需要处理的业务量较大，集群200w QPS，单台4-5w QPS，当时也在很多地方用来sync.Pool和bytes.Buffer做性能优化。

> 另外需要在这里说一些自己的感悟，如果我自己之前没有做过对性能优化相关的工作的话，那么我自己看这段源码可能也是一扫而过，知道在这里用了sync.Pool，但是sync.Pool到底起到什么作用（避免频繁分配内存导致的性能下降），可能从根本上还是没有那么透彻，看来技能还是得多多打磨。

接着分析是，使用for循环一直从conn中读出请求，然后进行处理。从代码猜的是，看来请求并不是一次都能读取完毕，可能会多次读取。

在这里出现了把我们最初定义的server封装成一个`serverHandler`的类型，该类型其实就是一个server的Handler或者DefaultServeMux，在serverHandler的`ServeHTTP(w, w.req)`函数定义如下，

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

可以在这里面看到，如果我们最开始的时候，server中的Handler如果是nil的话，就使用默认的DefaultServeMux。

我们在demo中，使用`http.ListenAndServe(":8080", nil)`启动服务时，第二个参数就是nil，所以我们在demo中，用到的就是默认的DefaultServeMux。

那么最后当调用`handler.ServeHTTP(rw, req)`时，DefaultServeMux.ServeHTTP会如何处理这些请求呢？

我们可以看到DefaultServeMux的定义是：

```go
// DefaultServeMux is the default ServeMux used by Serve.
var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux
```

其中`ServeMux`的定义是：

```go
type ServeMux struct {
	mu    sync.RWMutex
	m     map[string]muxEntry
	hosts bool // whether any patterns contain hostnames
}
```

我们这样就找到之前所解释的`ServeMux`的调用的地方了，也就是我们终于找到了我们想要分析的路由router的入口点了。

那么我们继续分析ServeMux.ServeHTTP都是如何做的。

```go
// ServeHTTP dispatches the request to the handler whose
// pattern most closely matches the request URL.
func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request) {
	// ...
	h, _ := mux.Handler(r)
	h.ServeHTTP(w, r)
}
```

啊哈，起码从注释中，我们可以看到ServeHTTP是把请求分发给最适配的处理函数。

这就是我们要分析的关键所在。

重点在于最后两句：

1. `h, _ := mux.Handler(r)`：返回一个Handler（Handler的定义是：A Handler responds to an HTTP request.），Handler就是对HTTP请求进行处理并作出返回的函数。

```go
// Handler returns the handler to use for the given request,
// consulting r.Method, r.Host, and r.URL.Path. It always returns
// a non-nil handler. 
func (mux *ServeMux) Handler(r *Request) (h Handler, pattern string) {
	// ...
	return mux.handler(host, r.URL.Path)
}
```

我们暂时不考虑HTTP CONNECT METHOD，剩下的主要的是`mux.handler(host, r.URL.Path)`。


```go
// handler is the main implementation of Handler.
// The path is known to be in canonical form, except for CONNECT methods.
func (mux *ServeMux) handler(host, path string) (h Handler, pattern string) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	// Host-specific pattern takes precedence over generic ones
	if mux.hosts {
		h, pattern = mux.match(host + path)
	}
	if h == nil {
		h, pattern = mux.match(path)
	}
	if h == nil {
		h, pattern = NotFoundHandler(), ""
	}
	return
}
```

做了下面操作，

1. 看mux中有没有指定hosts需要匹配满足的，如果需要，那么查找的时候，在路径前面添加host，这个host是请求的Host。如果找到匹配的Handler，那么返回该Handler。

2. 如果第1步没有找到。则去掉host，查找是否有匹配的Handler，如果有，返回。

3. 如果没有匹配的，那么返回一个特殊的Handler——NotFoundHandler()，该Handler的定义如下，也就是往Response中写入404。

```go
// NotFound replies to the request with an HTTP 404 not found error.
func NotFound(w ResponseWriter, r *Request) { Error(w, "404 page not found", StatusNotFound) }

// NotFoundHandler returns a simple request handler
// that replies to each request with a ``404 page not found'' reply.
func NotFoundHandler() Handler { return HandlerFunc(NotFound) }
```


我们往下分析path匹配的函数，`h, pattern = mux.match(path)`

```go
// Find a handler on a handler map given a path string.
// Most-specific (longest) pattern wins.
func (mux *ServeMux) match(path string) (h Handler, pattern string) {
	// Check for exact match first.
	v, ok := mux.m[path]
	if ok {
		return v.h, v.pattern
	}

	// Check for longest valid match.
	var n = 0
	for k, v := range mux.m {
		if !pathMatch(k, path) {
			continue
		}
		if h == nil || len(k) > n {
			n = len(k)
			h = v.h
			pattern = v.pattern
		}
	}
	return
}
```

从mux中，也即默认的DefaultServeMux，注册Handler map表中搜索，根据一个符合的返回。如果有多个handler都符合，那么返回path匹配最长路径的handler。


> 在这里我们可以看到http默认的mux是不区分http方法的，所以只要路径满足，不管request method是get、post、delete，都是会进入到同一个Handler中。


我们回顾到上面调用处理函数的地方，

```go
// ServeHTTP dispatches the request to the handler whose
// pattern most closely matches the request URL.
func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request) {
	// ...
	h, _ := mux.Handler(r)
	h.ServeHTTP(w, r)
}
```

我们从mux中获取到了一个Handler h，我们是使用`h.ServeHTTP(w, r)`来调用我们定义的请求处理函数。

我们定义的请求处理函数原型是：`func fooHandler(w http.ResponseWriter, r *http.Request) `

并且Handler interface的定义也是下面这样，

```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

也要求Handler，也即是我们的处理函数需要满足：ServeHTTP(ResponseWriter, *Request)，然而我们的函数并没有定义ServeHTTP函数，这是怎么一回事？

奇妙之处就在于注册的时候，

当我们用`http.HandleFunc("/foo", fooHandler)`注册时，

```go
func HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	// ...
	mux.Handle(pattern, HandlerFunc(handler))
}
```

在HandleFunc中，会把我们自定义的handler（也就是fooHandler）强制类型转换为`HandlerFunc(handler)`。

而HandlerFunc的定义如下，

```go
// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
	f(w, r)
}
```

HandlerFunc就是为了适配自定义的函数为标准的Handler。


-----

前面的流程串一下就是：

1. 我们启动服务并监听一个地址，等待请求

2. 当来了一个请求后，我们会启动一个goroutine对这个请求进行处理

3. 处理的这个请求的方式，也即我们是怎么查找这个请求的对应的处理函数是根据Handler.ServeHTTP来规定的，这个也就是我们要找的router。

4. 由于我们没有定义自己的router，所以我们使用了默认的DefaultServeMux

5. 从DefaultServeMux注册的所有Handler中，找出最合适Handler进行处理（匹配且路径最长）