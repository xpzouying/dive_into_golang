# golang默认的http router


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

