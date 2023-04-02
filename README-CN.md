中文 | [English](./README.md)
<div align="center">

# Shermie-Proxy

</div>

<div align="center">

![GitHub User's stars](https://img.shields.io/github/stars/kxg3030?style=social)
![GitHub go.mod Go version (branch & subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/kxg3030/shermie-proxy/master)
![GitHub](https://img.shields.io/github/license/kxg3030/shermie-proxy)
![GitHub search hit counter](https://img.shields.io/github/search/kxg3030/shermie-proxy/start)
![GitHub release (by tag)](https://img.shields.io/github/downloads/kxg3030/shermie-proxy/v1.1/total)
![GitHub commit activity](https://img.shields.io/github/commit-activity/w/kxg3030/shermie-proxy)
</div>

# 功能

- 支持多种协议的数据接收发送，只需要监听一个端口
- 支持数据拦截和自定义修改
- 自动识别入站协议，按照消息头识别不同的协议进行处理
- 支持添加上级代理

TODO：

- tcp连接复用

# 使用

- 安装
```go
go get github.com/kxg3030/shermie-proxy
```

- 监听服务
```go
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"github.com/kxg3030/shermie-proxy/Core"
	"github.com/kxg3030/shermie-proxy/Core/Websocket"
	"github.com/kxg3030/shermie-proxy/Log"
	"io"
	"net/http"
	"strings"
)

func init() {
	// 初始化日志
	Log.NewLogger().Init()
	// 初始化根证书
	err := Core.NewCertificate().Init()
	if err != nil {
		Log.Log.Println("初始化根证书失败：" + err.Error())
		return
	}
}

func main() {
	port  := flag.String("port", "9090", "listen port")
	nagle := flag.Bool("nagle", true, "connect remote use nagle algorithm")
	proxy := flag.String("proxy", "", "prxoy remote host")
	to    := flag.String("to", "", "tcp remote host")
	flag.Parse()
	if *port == "0" {
		Log.Log.Fatal("port required")
		return
	}
	// 启动服务
	s := Core.NewProxyServer(*port, *nagle, *proxy, *to)
	
	// 注册tcp连接事件
	s.OnTcpConnectEvent = func(conn net.Conn) {

	}
	// 注册tcp关闭事件
	s.OnTcpCloseEvent = func(conn net.Conn) {

	}
	s.OnHttpRequestEvent = func(message []byte, request *http.Request, resolve Core.ResolveHttpRequest, conn net.Conn) bool{
		Log.Log.Println("HttpRequestEvent：" + conn.RemoteAddr().String())
		resolve(message, request)
		return true
	}
	// 注册http服务器响应事件函数
	s.OnHttpResponseEvent = func(body []byte, response *http.Response, resolve Core.ResolveHttpResponse, conn net.Conn) bool{
		mimeType := response.Header.Get("Content-Type")
		if strings.Contains(mimeType, "json") {
			Log.Log.Println("HttpResponseEvent：" + string(body))
		}
		// 可以在这里做数据修改
		resolve(body, response)
		return true
	}

	// 注册socket5服务器推送消息事件函数
	s.OnSocks5ResponseEvent = func(message []byte, resolve Core.ResolveSocks5, conn net.Conn) (int, error) {
		Log.Log.Println("Socks5ResponseEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册socket5客户端推送消息事件函数
	s.OnSocks5RequestEvent = func(message []byte, resolve Core.ResolveSocks5, conn net.Conn) (int, error) {
		Log.Log.Println("Socks5RequestEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册ws客户端推送消息事件函数
	s.OnWsRequestEvent = func(msgType int, message []byte, resolve Core.ResolveWs, conn net.Conn) error {
		Log.Log.Println("WsRequestEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(msgType, message)
	}

	// 注册ws服务器推送消息事件函数
	s.OnWsResponseEvent = func(msgType int, message []byte, resolve Core.ResolveWs, conn net.Conn) error {
		Log.Log.Println("WsResponseEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(msgType, message)
	}

	// 注册tcp服务器推送消息事件函数
	s.OnTcpClientStreamEvent = func(message []byte, resolve Core.ResolveTcp, conn net.Conn) (int, error) {
		Log.Log.Println("TcpClientStreamEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册tcp服务器推送消息事件函数
	s.OnTcpServerStreamEvent = func(message []byte, resolve Core.ResolveTcp, conn net.Conn) (int, error) {
		Log.Log.Println("TcpServerStreamEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	_ = s.Start()
}
```
- 参数


    --port:代理服务监听的端口,默认为9090


    --to:代理tcp服务时,目的服务器的ip和端口,默认为0,仅tcp代理使用


    --proxy:上级代理地址


    --nagle:是否开启nagle数据合并算法,默认true

# 交流

<div align="center">
	<a href="https://t.zsxq.com/0allV9fqi" style="font-size:16px;font-weight:bold">点击加入我的星球</a>
</div>
<br/>
<div align="center">
	<a href="https://t.zsxq.com/0allV9fqi" style="font-size:16px;font-weight:bold">QQ群：931649621</a>
</div>