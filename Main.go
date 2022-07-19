package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"github/shermie-proxy/Core"
	"github/shermie-proxy/Core/Websocket"
	"github/shermie-proxy/Log"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var port *string

func init() {
	port = flag.String("port", "9090", "listen port")
	flag.Parse()
	if *port == "0" {
		log.Fatal("port required")
	}
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
	// 启动服务
	s := Core.NewProxyServer(*port)
	// 注册http事件函数
	s.OnRequestEvent = func(request *http.Request) {

	}
	// 注册http事件函数
	s.OnResponseEvent = func(response *http.Response) {
		contentType := response.Header.Get("Content-Type")
		var reader io.Reader
		if strings.Contains(contentType, "json") {
			reader = bufio.NewReader(response.Body)
			if header := response.Header.Get("Content-Encoding"); header == "gzip" {
				reader, _ = gzip.NewReader(response.Body)
			}
			body, _ := io.ReadAll(reader)
			fmt.Println(string(body))
		}
	}
	// 注册ws事件函数
	s.OnServerPacketEvent = func(msgType int, message []byte, clientConn *Websocket.Conn, resolve Core.ResolveWs) error {
		fmt.Println("服务器向浏览器响应数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
		return clientConn.WriteMessage(msgType,message)
	}
	s.OnClientPacketEvent = func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve Core.ResolveWs) error {
		fmt.Println("浏览器向服务器发送数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
		return resolve(msgType,message,tartgetConn)
	}
	_ = s.Start()
}
