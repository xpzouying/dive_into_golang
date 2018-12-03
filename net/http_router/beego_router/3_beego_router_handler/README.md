# beego router系列3:搜索路径规则

## 简介

前面已经分析了

1. 如何启动beego的http server
2. 如何注册router
3. 如何搜索一个已经注册好的router，比如对于`/`处理的router

我们还有下面几部分需要分析，

1. 多级request url如何搜索对应的router：比如`/hello`,`/say/hi`,`/say/bye`
2. 带有通配符的路径：`/:user`、`/:id/:age`
3. middleware：如何记录请求日志

## 分析

### 多级request url搜索的过程

#### 注册

当我们注册`/say/hi`、`/say/bye`、`/hello`三个路由路径时，在AddRouter函数中会对请求路径进行字符串的切分。

```go
// AddRouter call addseg function
func (t *Tree) AddRouter(pattern string, runObject interface{}) {
    t.addseg(splitPath(pattern), runObject, nil, "")
}
```

对于`splitPath(pattern)`分别为：`[]string{"say", "hi"}`、`[]string{"say", "hi"}`、和`[]string{"hello"}`。

`runObject interface{}`：为`ControllerInfo`，其中封装了FilterFunc（即beego版的http handle func）。

`addseg`方法在注册时区分了很多种情况，注册的时候，区分了`普通字符串`、`通配符`、`正则表达式`的情况。



```go

// "/"
// "admin" ->
func (t *Tree) addseg(segments []string, route interface{}, wildcards []string, reg string) {
	if len(segments) == 0 {
		// ...
	} else {
		seg := segments[0]
        // /hello --> false, nil, ""
		iswild, params, regexpStr := splitSegment(seg)
		// if it's ? meaning can igone this, so add one more rule for it
		if len(params) > 0 && params[0] == ":" {
			// ...
		}
		//Rule: /login/*/access match /login/2009/11/access
		//if already has *, and when loop the access, should as a regexpStr
		if !iswild && utils.InSlice(":splat", wildcards) {
			// * 匹配
		}
		//Rule: /user/:id/*
		if seg == "*" && len(wildcards) > 0 && reg == "" {
			// ...
		}
		if iswild {
			// 如果是通配符情况
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

// "admin" -> false, nil, ""
// ":id" -> true, [:id], ""
// "?:id" -> true, [: :id], ""        : meaning can empty
// ":id:int" -> true, [:id], ([0-9]+)
// ":name:string" -> true, [:name], ([\w]+)
// ":id([0-9]+)" -> true, [:id], ([0-9]+)
// ":id([0-9]+)_:name" -> true, [:id :name], ([0-9]+)_(.+)
// "cms_:id_:page.html" -> true, [:id_ :page], cms_(.+)(.+).html
// "cms_:id(.+)_:page.html" -> true, [:id :page], cms_(.+)_(.+).html
// "*" -> true, [:splat], ""
// "*.*" -> true,[. :path :ext], ""      . meaning separator
func splitSegment(key string) (bool, []string, string) {
	// ...
}
```

在`splitSegment`函数中，对pattern path做出了各种处理。

这一章仅仅分析：普通的情况`/hello`，这里返回的是`false, nil, ""`。

对于通配符、正则表达式留在以后分析。

由于我们是调用第一次，所以走到下面分支，

```go
var subTree *Tree
if subTree == nil {
	subTree = NewTree()
	subTree.prefix = seg
	t.fixrouters = append(t.fixrouters, subTree)
}
subTree.addseg(segments[1:], route, wildcards, reg)
```



创建了一颗新的Tree进行保存，Tree的结构如下，

```go
// Tree has three elements: FixRouter/wildcard/leaves
// fixRouter stores Fixed Router
// wildcard stores params
// leaves store the endpoint information
type Tree struct {
	//prefix set for static router
	prefix string
	//search fix route first
	fixrouters []*Tree
	//if set, failure to match fixrouters search then search wildcard
	wildcard *Tree
	//if set, failure to match wildcard search
	leaves []*leafInfo
}
```

包含了下面3个部分：

- fixRouter：保存修正的router信息
- wildcard：保存参数
- leaves：保存endpoint信息



所以在添加router信息的时候，

创建了一颗新的Tree，其中prefix为：`hello`，fixrouters为这棵树。最后subTree.addseg又开始添加url第二级的字符串了。



总结一下当我们注册一个`GET /hello`时，我们其实是，

1. 使用了一个`ControllerRegister`的结构体作为http的Handler，其中实现了`ServeHTTP`方法；
2. 注册时，结构体中有个成员为`routers map[string]*Tree`，其中key为对应的method，value为Tree结构。在这里就是在routers的map中，增加了key为get的一个值，值为`*Tree`。
3. 这个Tree添加了对应的路由信息；其中路由信息是一棵子树，它的prefix为`hello`。这棵子树被挂在根节点（树根）的`fixrouters`上面，fixrouters是一个子树的列表。