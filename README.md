#### 一、项目
> 使用go开发的代理工具，目前支持http、https、ws、wss、tcp、socket5等协议

- 支持多种协议的数据接收发送，只需要监听一个端口
- 支持数据拦截和自定义修改
- 自动识别入站协议，按照消息头识别不同的协议进行处理
- 支持添加代理

#### 二、使用

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
	port := flag.String("port", "9090", "listen port")
	nagle := flag.Bool("nagle", true, "connect remote use nagle algorithm")
	proxy := flag.String("proxy", "0", "tcp prxoy remote host")
	flag.Parse()
	if *port == "0" {
		Log.Log.Fatal("port required")
		return
	}
	// 启动服务
	s := Core.NewProxyServer(*port, *nagle, *proxy)
	// 注册http客户端请求事件函数
	s.OnHttpRequestEvent = func(request *http.Request) {

	}
	// 注册http服务器响应事件函数
	s.OnHttpResponseEvent = func(response *http.Response) {
		contentType := response.Header.Get("Content-Type")
		var reader io.Reader
		if strings.Contains(contentType, "json") {
			reader = bufio.NewReader(response.Body)
			if header := response.Header.Get("Content-Encoding"); header == "gzip" {
				reader, _ = gzip.NewReader(response.Body)
			}
			body, _ := io.ReadAll(reader)
			Log.Log.Println("HttpResponseEvent：" + string(body))
		}
	}
	// 注册socket5服务器向客户端推送消息事件函数
	s.OnSocket5ResponseEvent = func(message []byte) {
		Log.Log.Println("Socket5ResponseEvent：" + string(message))
	}
	// 注册socket5客户端向服务器推送消息事件函数
	s.OnSocket5RequestEvent = func(message []byte) {
		Log.Log.Println("Socket5RequestEvent：" + string(message))
	}
	// 注册ws服务器向客户端推送消息事件函数
	s.OnWsRequestEvent = func(msgType int, message []byte, clientConn *Websocket.Conn, resolve Core.ResolveWs) error {
		Log.Log.Println("WsRequestEvent：" + string(message))
		return clientConn.WriteMessage(msgType, message)
	}
	// 注册ws客户端向服务器推送消息事件函数
	s.OnWsResponseEvent = func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve Core.ResolveWs) error {
		Log.Log.Println("WsResponseEvent：" + string(message))
		return resolve(msgType, message, tartgetConn)
	}
	_ = s.Start()
}
}
```
- 参数

    --port:代理服务监听的端口,默认为9090

    --proxy:代理tcp服务时,目的服务器的ip和端口,默认为0,仅tcp代理使用

    --nagle:是否开启nagle数据合并算法,默认true