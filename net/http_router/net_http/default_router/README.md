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