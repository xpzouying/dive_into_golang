package main

import (
	"bufio"
	"fmt"
)

type ZYWriter1 struct{}

func (ZYWriter1) Write(p []byte) (n int, err error) {
	fmt.Printf("ZYWriter1 write %d byte: %q\n", len(p), p)
	return len(p), nil
}

func bufferAndUnBufferDemo() {
	fmt.Println("---bufferAndUnBufferDemo---")

	fmt.Println("unbuffer I/O")
	w1 := ZYWriter1{}
	w1.Write([]byte{'z'})
	w1.Write([]byte{'o'})
	w1.Write([]byte{'u'})

	fmt.Println("buffer I/O")
	w2 := new(ZYWriter1)
	bw := bufio.NewWriterSize(w2, 2)
	bw.Write([]byte{'z'})
	bw.Write([]byte{'o'})
	bw.Write([]byte{'u'})

}

func main() {
	bufferAndUnBufferDemo()
}
