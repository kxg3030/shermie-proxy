package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Core"
	"github.com/kxg3030/shermie-proxy/Core/Websocket"
	"github.com/kxg3030/shermie-proxy/Log"
	"io"
	"net/http"
	"strconv"
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
	// 注册http事件函数
	s.OnHttpRequestEvent = func(request *http.Request) {
		//fmt.Println("http请求地址：" + request.URL.Host)
	}
	// 注册http事件函数
	s.OnHttpResponseEvent = func(response *http.Response) {
		contentType := response.Header.Get("Content-Type")
		var reader io.Reader
		if strings.Contains(contentType, "json") {
			reader = bufio.NewReader(response.Body)
			if header := response.Header.Get("Content-Encoding"); header == "gzip" {
				reader, _ = gzip.NewReader(response.Body)
			}
			body, _ := io.ReadAll(reader)
			fmt.Println("http返回数据：" + string(body))
		}
	}
	// 注册socket5服务器向客户端推送消息事件函数
	s.OnSocket5ResponseEvent = func(message []byte) {
		fmt.Println("socket5服务器发送数据", message)
	}
	// 注册socket5客户端向服务器推送消息事件函数
	s.OnSocket5RequestEvent = func(message []byte) {
		fmt.Println("socket5客户端发送数据", message)
	}
	// 注册ws服务器向客户端推送消息事件函数
	s.OnWsRequestEvent = func(msgType int, message []byte, clientConn *Websocket.Conn, resolve Core.ResolveWs) error {
		fmt.Println("服务器向浏览器响应数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
		return clientConn.WriteMessage(msgType, message)
	}
	// 注册ws客户端向服务器推送消息事件函数
	s.OnWsResponseEvent = func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve Core.ResolveWs) error {
		fmt.Println("浏览器向服务器发送数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
		return resolve(msgType, message, tartgetConn)
	}
	_ = s.Start()
}
