# dive into beego/router

上一次分析了默认的http sever以及router的工作方式。

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


