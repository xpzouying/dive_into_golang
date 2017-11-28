# c语言的tcp网络编程分析

Link:
	- [https://xpzouying.github.io/post/03_c_tcp_program/](https://xpzouying.github.io/post/03_c_tcp_program/)

参考《Unix Network Programming, Volume 1: The Sockets Networking API (3rd Edition)》

网络模型

![TCP Server/Client](http://cdncontribute.geeksforgeeks.org/wp-content/uploads/Socket-Programming-in-C-C--300x300.jpg)


### client端
**主要功能：**

client通过tcp访问server，获得服务器时间。

主要代码如下：

```c++
int main(int argc, char **argv)
{
	int					sockfd, n;
	char				recvline[MAXLINE + 1];
	struct sockaddr_in	servaddr;

	if (argc != 2)
		err_quit("usage: a.out <IPaddress>");

	if ( (sockfd = socket(AF_INET, SOCK_STREAM, 0)) < 0)
		err_sys("socket error");

	bzero(&servaddr, sizeof(servaddr));
	servaddr.sin_family = AF_INET;
	servaddr.sin_port   = htons(13);	/* daytime server */
	if (inet_pton(AF_INET, argv[1], &servaddr.sin_addr) <= 0)
		err_quit("inet_pton error for %s", argv[1]);

	if (connect(sockfd, (SA *) &servaddr, sizeof(servaddr)) < 0)
		err_sys("connect error");

	while ( (n = read(sockfd, recvline, MAXLINE)) > 0) {
		recvline[n] = 0;	/* null terminate */
		if (fputs(recvline, stdout) == EOF)
			err_sys("fputs error");
	}
	if (n < 0)
		err_sys("read error");

	exit(0);
}
```

**代码解析：**

1. `sockfd = socket(AF_INET, SOCK_STREAM, 0)` 获得一个socket descriptor，类似于一个file-handle。传入参数：第1个AF_INET表示IPv4协议，如果AF_INET6则表示IPv6协议。第2个参数表示传输类型：SOCK_STREAM表示TCP，SOCK_DGRAM表示UDP。第3个配置IP协议value，一般为0。
2. `inet_pton(AF_INET, argv[1], &servaddr.sin_addr)` 是一个IP地址转换成packed address函数。第1个参数是指协议类型，AF_INET或者IF_INET6；第2个参数是地址的字符串形式，比如"192.168.1.100"，第3个参数是将第2个参数转换成的packed address保存。
3. `connect(sockfd, (SA *) &servaddr, sizeof(servaddr))` 通过第一步中创建的socket descriptor建立socket链接。
4. `read(sockfd, recvline, MAXLINE)` 从已经打开的sockfd读取数据。
5. `fputs(recvline, stdout)` 将字符串写到stdout中。



### server端

**主要功能**

server监听一个socket地址，收到client的请求后，将当前时间返回给client。

```c++
int main(int argc, char **argv)
{
	int					listenfd, connfd;
	struct sockaddr_in	servaddr;
	char				buff[MAXLINE];
	time_t				ticks;

	listenfd = Socket(AF_INET, SOCK_STREAM, 0);

	bzero(&servaddr, sizeof(servaddr));
	servaddr.sin_family      = AF_INET;
	servaddr.sin_addr.s_addr = htonl(INADDR_ANY);
	servaddr.sin_port        = htons(13);	/* daytime server */

	Bind(listenfd, (SA *) &servaddr, sizeof(servaddr));

	Listen(listenfd, LISTENQ);

	for ( ; ; ) {
		connfd = Accept(listenfd, (SA *) NULL, NULL);

        ticks = time(NULL);
        snprintf(buff, sizeof(buff), "%.24s\r\n", ctime(&ticks));
        Write(connfd, buff, strlen(buff));

		Close(connfd);
	}
}
```


**代码解析：**

1. `listenfd = Socket(AF_INET, SOCK_STREAM, 0);` 创建一个IPv4、基于TCP连接的socket，返回socket descriptor。
2. `servaddr.sin_addr.s_addr = htonl(INADDR_ANY);` htonl将32位的主机字符顺序转换成网络字符顺序。`INADDR_ANY`表示`0.0.0.0`，监听所有的地址；在Ubuntu 16.04，`INADDR_ANY`定义在`/usr/include/linux/in.h`中：`#define INADDR_ANY ((unsigned long int) 0x00000000)`。
3. `Bind(listenfd, (SA *) &servaddr, sizeof(servaddr));` 将sockfd绑定在servaddr地址上。
4. `Listen(listenfd, LISTENQ);` 等待sockfd收到的连接。第二个参数表示同时能处理的最大链接要求，如果达到最大连接个数，会返回client：ECONNREFUSED的错误。
5. `connfd = Accept(listenfd, (SA *) NULL, NULL);` 从listen状态的sockfd接收连接。
6. `Write(connfd, buff, strlen(buff));` 将数据buff写入到接收的connfd中。
7. `Close(connfd);` 关闭连接。