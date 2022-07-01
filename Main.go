package main

import (
	"flag"
	"fmt"
	"github/shermie-proxy/Core"
	"github/shermie-proxy/Log"
	"log"
	"net/http"
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
	// 注册事件函数
	s.OnRequestEvent = func(request *http.Request) {
		fmt.Println(request.RequestURI)
	}
	s.OnResponseEvent = func(response *http.Response) {
		fmt.Println(response.Header)
	}
	_ = s.Start()
}
