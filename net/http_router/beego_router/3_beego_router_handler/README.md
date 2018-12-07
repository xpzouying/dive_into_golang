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

一句话总结就是：有一张注册表，是map结构，每一种类型method对应会有一棵树Tree。

#### 查找router

通过上一节的注册后，我们如何查找到对应的router呢？

我们直接进入到我们的Handler，也即`ControllerRegister`的`ServeHTTP`，

```go
// Implement http.Handler interface.
func (p *ControllerRegister) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	// ...
    routerInfo, findRouter = p.FindRouter(context)
    // ...
}

// FindRouter Find Router info for URL
func (p *ControllerRegister) FindRouter(context *beecontext.Context) (routerInfo *ControllerInfo, isFind bool) {
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



1. 找到ControllerRegister中`routers["get"]`对应的Tree。我们有注册"get"对应的Tree，所以是可以找到这棵Tree。
2. 通过`runObject := t.Match(urlPath, context)`：找到对应的路由信息。我们看看Match的过程。

```go
// Match router to runObject & params
func (t *Tree) Match(pattern string, ctx *context.Context) (runObject interface{}) {
	if len(pattern) == 0 || pattern[0] != '/' {
		return nil
	}
	w := make([]string, 0, 20)
	return t.match(pattern[1:], pattern, w, ctx)
}

func (t *Tree) match(treePattern string, pattern string, wildcardValues []string, ctx *context.Context) (runObject interface{}) {
	if len(pattern) > 0 {
		i := 0
		for ; i < len(pattern) && pattern[i] == '/'; i++ {
		}
		pattern = pattern[i:]
	}
	// Handle leaf nodes:
	if len(pattern) == 0 {
		for _, l := range t.leaves {
			if ok := l.match(treePattern, wildcardValues, ctx); ok {
				return l.runObject
			}
		}
		if t.wildcard != nil {
			for _, l := range t.wildcard.leaves {
				if ok := l.match(treePattern, wildcardValues, ctx); ok {
					return l.runObject
				}
			}
		}
		return nil
	}
	var seg string
	i, l := 0, len(pattern)
	for ; i < l && pattern[i] != '/'; i++ {
	}
	if i == 0 {
		seg = pattern
		pattern = ""
	} else {
		seg = pattern[:i]
		pattern = pattern[i:]
	}
	for _, subTree := range t.fixrouters {
		if subTree.prefix == seg {
			if len(pattern) != 0 && pattern[0] == '/' {
				treePattern = pattern[1:]
			} else {
				treePattern = pattern
			}
			runObject = subTree.match(treePattern, pattern, wildcardValues, ctx)
			if runObject != nil {
				break
			}
		}
	}
	if runObject == nil && len(t.fixrouters) > 0 {
		// Filter the .json .xml .html extension
		for _, str := range allowSuffixExt {
			if strings.HasSuffix(seg, str) {
				for _, subTree := range t.fixrouters {
					if subTree.prefix == seg[:len(seg)-len(str)] {
						runObject = subTree.match(treePattern, pattern, wildcardValues, ctx)
						if runObject != nil {
							ctx.Input.SetParam(":ext", str[1:])
						}
					}
				}
			}
		}
	}
	if runObject == nil && t.wildcard != nil {
		runObject = t.wildcard.match(treePattern, pattern, append(wildcardValues, seg), ctx)
	}

	if runObject == nil && len(t.leaves) > 0 {
		wildcardValues = append(wildcardValues, seg)
		start, i := 0, 0
		for ; i < len(pattern); i++ {
			if pattern[i] == '/' {
				if i != 0 && start < len(pattern) {
					wildcardValues = append(wildcardValues, pattern[start:i])
				}
				start = i + 1
				continue
			}
		}
		if start > 0 {
			wildcardValues = append(wildcardValues, pattern[start:i])
		}
		for _, l := range t.leaves {
			if ok := l.match(treePattern, wildcardValues, ctx); ok {
				return l.runObject
			}
		}
	}
	return runObject
}
```



**Match**过程：

1. `t.Match`函数：我们的输入参数pattern是http request的路径，在我们这个例子为http.Request.Path，也即`/hello`。

2. `t.match`调用时输入为：pattern: /hello。

3. 主要过程在`t.match`函数里面，重点分析该函数。

   1. `if len(pattern) > 0`作用是：删掉http request path起始的`/`符号。如果去掉的其实的`/`符号后，只有两种情况出现了，1)仍然有字符，那么表示还有路径可以匹配，后续递归调用t.Match继续匹配；2)如果没有字符了，那么就是叶子节点，那么判断该叶子节点是不是有我们注册的处理函数。

      > 不太明白作者为什么要自己写一个for循环进行操作，而不选用`strings.Trim`函数直接操作；

   2. 由于我们的pattern为`/hello`，所以经过上面操作后，pattern变成了`hello`。

   3. `if len(pattern) == 0`作用是：如果是pattern为空了，也即表示为一个叶子节点，通过叶子节点最终判断是否有注册的router；首次进入的时候，该if分支跳过；

   4. 继续，seg会赋值为`hello`，pattern会赋值为空字符串；

   5. `for _, subTree := range t.fixrouters`会对注册的所有的router tree列表进行遍历，我们通过注册的prefix进行匹配，prefix为之前注册的路径字符串按照`/`进行分片注册的，所以我们已经对`hello`进行注册了。

   6. 所以我们进行`subTree.match`判断是否匹配，其中入参treePattern、pattern为空字符串。该函数其实就是通过递归调用搜索。

   7. 从函数开头继续：`if len(pattern) > 0`，该if跳过，长度已经为0。

   8. 由于长度为0，所以进入`if len(pattern) == 0`分支，进行树的叶子节点搜索是否匹配，通过`l.match(treePattern, wildcardValues, ctx)`进行判断，如果找到的话，则返回叶子节点上的`runObject`，也就是我们注册时的http处理函数。

4. 结束递归调用，返回到`ServeHTTP`函数中：`routerInfo, findRouter = p.FindRouter(context)`。后面使用routerInfo进行函数执行：`routerInfo.runFunction(context)`，也即是执行了我们注册时的处理函数。

到此为止，我们通过对一个简单的`/hello`，分析了beego中对于http处理函数的注册以及调用的过程。

