package main

import (
	"flag"
	"github.com/kxg3030/shermie-proxy/Core"
	"github.com/kxg3030/shermie-proxy/Log"
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
	proxy := flag.String("proxy", "", "prxoy remote host")
	to := flag.String("to", "", "tcp remote host")
	flag.Parse()
	if *port == "0" {
		Log.Log.Fatal("port required")
		return
	}
	// 启动服务
	s := Core.NewProxyServer(*port, *nagle, *proxy, *to)

	// 注册http客户端请求事件函数
	s.OnHttpRequestEvent = func(body []byte, request *http.Request, resolve Core.ResolveHttpRequest) {
		mimeType := request.Header.Get("Content-Type")
		if strings.Contains(mimeType, "json") {
			Log.Log.Println("HttpRequestEvent：" + string(body))
		}
		// 可以在这里做数据修改
		resolve(body, request)
	}

	// 注册http服务器响应事件函数
	s.OnHttpResponseEvent = func(body []byte, response *http.Response, resolve Core.ResolveHttpResponse) {
		mimeType := response.Header.Get("Content-Type")
		if strings.Contains(mimeType, "json") {
			Log.Log.Println("HttpResponseEvent：" + string(body))
		}
		// 可以在这里做数据修改
		resolve(body, response)
	}

	// 注册socket5服务器推送消息事件函数
	s.OnSocket5ResponseEvent = func(message []byte, resolve Core.ResolveSocks5) (int, error) {
		Log.Log.Println("Socket5ResponseEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册socket5客户端推送消息事件函数
	s.OnSocket5RequestEvent = func(message []byte, resolve Core.ResolveSocks5) (int, error) {
		Log.Log.Println("Socket5RequestEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册ws客户端推送消息事件函数
	s.OnWsRequestEvent = func(msgType int, message []byte, resolve Core.ResolveWs) error {
		Log.Log.Println("WsRequestEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(msgType, message)
	}

	// 注册ws服务器推送消息事件函数
	s.OnWsResponseEvent = func(msgType int, message []byte, resolve Core.ResolveWs) error {
		Log.Log.Println("WsResponseEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(msgType, message)
	}

	// 注册tcp服务器推送消息事件函数
	s.OnTcpClientStreamEvent = func(message []byte, resolve Core.ResolveTcp) (int, error) {
		Log.Log.Println("TcpClientStreamEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	// 注册tcp服务器推送消息事件函数
	s.OnTcpServerStreamEvent = func(message []byte, resolve Core.ResolveTcp) (int, error) {
		Log.Log.Println("TcpServerStreamEvent：" + string(message))
		// 可以在这里做数据修改
		return resolve(message)
	}

	_ = s.Start()
}
