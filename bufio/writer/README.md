# bufio.Writer介绍

`bufio`实现了I/O的缓存，为`io.Reader`或`io.Writer`提供I/O缓存。

# 为什么需要bufio？

以`write()`操作为例。

对于没有缓存的I/O操作，进行一次write()操作需要将用户空间(user space)中的数据传输到内核空间(kernel space)后，调用`write()`系统调用完成write操作，然后返回用户空间。

如果每个字节(single byte)都需要重复上述过程，那么写入过程就非常低效率。

为了解决该问题，就引入了用户空间的I/O缓存（user buffered I/O）。在用户空间中分配一块缓存区用来临时保存数据，当缓存写满后，再调用`write()`系统调用来写数据，从而提供效率。

Golang中bufio实现的功能就是类似的功能。为某个`io.Writer`或者`io.Reader`提供缓存。

# bufio包中的主要类型
	- type ReadWriter
	- type Reader
	- type Scanner
	- type Writer （本文介绍的对象）


# type bufio.Writer

为`io.Writer`对象提供缓存。

```go
type  Writer  struct {
	err error
	buf []byte	// 缓存空间
	n   int 	// 统计已经缓存的数据个数
	wr  io.Writer  // 底层的io.Writer对象
}
```
`Writer`为一个结构体，定义了一块缓存，为wr对象提供缓存，err、n是当前缓存的中间状态，其中n表示当前缓存的数据个数，用来统计缓存区的缓存个数和计算剩余缓存空间大小。


# 使用说明

`bufio`提供来2个工厂函数获得`Writer`

1. [func NewWriter(w io.Writer) *Writer](https://golang.org/pkg/bufio/#NewWriter)
2. [func NewWriterSize(w io.Writer, size int) *Writer](https://golang.org/pkg/bufio/#NewWriterSize)


看源码，`bufio/bufio.go`

```go
func  NewWriter(w io.Writer) *Writer {
    return  NewWriterSize(w, defaultBufSize)
}
```
`NewWriter(w io.Writer)`为传入参数w产生一个缓存对象Writer。其内部实现也是调用了`NewWriterSize()`来产生Writer对象。其中`defaultBufSize`为默认缓存空间大小4096。

```go
const defaultBufSize  =  4096
```
`NewWriterSize(w io.Writer, size int)`为传入参数w产生一个大小size的缓存对象Writer。如果传入的size<=0，则使用默认的缓存大小4096。

```go
return  &Writer{buf: make([]byte, size), wr: w}
```
一次性分配size大小的缓存区，当前已经存放的缓存大小保存在n成员中，初始化为0。


## 重要函数

### [func (b *Writer) Available() int](https://golang.org/pkg/bufio/#Writer.Available)

返回buffer中还有多少bytes可用，`len(b.buf) - b.n`。


### [func (b *Writer) Buffered() int](https://golang.org/pkg/bufio/#Writer.Buffered)

返回buffer中已经使用了多少bytes，`b.n`。


### [func (b *Writer) Flush() error](https://golang.org/pkg/bufio/#Writer.Flush)

将缓存中的数据写入到底层io.Writer。

```go
// Flush writes any buffered data to the underlying io.Writer.
func (b *Writer) Flush() error {
    if b.err !=  nil {
        return b.err
    }
    if b.n ==  0 {
        // 如果缓存区中没有数据，直接返回
        return  nil
    }

    // 将缓存数据b.n个bytes写入下一层的io.Writer对象中
    n, err  := b.wr.Write(b.buf[0:b.n])
    if n < b.n && err ==  nil {
        err  = io.ErrShortWrite
    }

    if err !=  nil {
        if n > 0  && n < b.n {
            copy(b.buf[0:b.n-n], b.buf[n:b.n])
        }
        b.n -= n
        b.err  = err
        return err
    }
    b.n  =  0
    return  nil
}
```

### [func (b *Writer) ReadFrom(r io.Reader) (n int64, err error)](https://golang.org/pkg/bufio/#Writer.ReadFrom)

从io.Reader中读取数据。实现了io.ReaderFrom接口。

```go
func (b *Writer) ReadFrom(r io.Reader) (n int64, err error) {
    if b.Buffered() ==  0 {
        if  w, ok  := b.wr.(io.ReaderFrom); ok {
            return w.ReadFrom(r)
        }
    }

    var  m  int
    for {
        if b.Available() ==  0 {
            if  err1  := b.Flush(); err1 !=  nil {
                return n, err1
            }
        }

        nr  :=  0
        // maxConsecutiveEmptyReads = 100
        for nr < maxConsecutiveEmptyReads {
            m, err  = r.Read(b.buf[b.n:])
            if m !=  0  || err !=  nil {
                break
            }
            nr++
        }

        if nr == maxConsecutiveEmptyReads {
            return n, io.ErrNoProgress
        }
        b.n += m
        n +=  int64(m)
        if err !=  nil {
            break
        }
    }

    if err == io.EOF {
        // If we filled the buffer exactly, flush preemptively.
        if b.Available() ==  0 {
            err  = b.Flush()
        } else {
            err  =  nil
        }
    }
    return n, err
}
```
1. 如果缓存已满，那么先将缓存的数据写入(Flush)到底层io.Writer中。
2. 尝试从函数入参r中读取数据。


### [func (b *Writer) Reset(w io.Writer)](https://golang.org/pkg/bufio/#Writer.Reset)

重置底层中的io.Writer。会清空当前Writer中的状态，包括清空所有没有Flush的缓存数据、清除错误标示err成员。

### [func (b *Writer) Write(p \[\] byte) (nn int, err error)](https://golang.org/pkg/bufio/#Writer.Write)

将p写入缓存。

## 实例

自定义类型`ZYWriter1`，实现`io.Writer`接口。

```go
type  ZYWriter1  struct{}

func (ZYWriter1) Write(p []byte) (n int, err error) {
    fmt.Printf("ZYWriter1 write %d byte\n", len(p))
    return  len(p), nil

}
```

**无缓存I/O**

```go
    w1  := ZYWriter1{}
    w1.Write([]byte{'z'})
    w1.Write([]byte{'o'})
    w1.Write([]byte{'u'})

// 输出
// ZYWriter1 write 1 byte: "z"
// ZYWriter1 write 1 byte: "o"
// ZYWriter1 write 1 byte: "u"
```
在无缓存的情况下，对于每一次写入，都会直接写入到实际io.Writer中。

有缓存I/O：

```go
    w2  :=  new(ZYWriter1)
    bw  := bufio.NewWriterSize(w2, 2)
    bw.Write([]byte{'z'})
    bw.Write([]byte{'o'})
    bw.Write([]byte{'u'})

// 输出
// ZYWriter1 write 2 byte: "zo"
```
在有缓存的情况下，该例中，定义了长度为2的缓存Writer bw。
对于每一次的写入，都会先写入到缓存区，当缓存区写满，会Flush到底层的实际的io.Writer中。
所以在输出中，只有当写满2 bytes字节时，才会真正地输出，调用`io.Writer`的Write()。


# 参考

-  [https://golang.org/pkg/bufio/](https://golang.org/pkg/bufio/)
-  [In C, what does buffering I/O or buffered I/O mean? @quora, by Robert Love](https://www.quora.com/In-C-what-does-buffering-I-O-or-buffered-I-O-mean/answer/Robert-Love-1)
