package main

import (
	"flag"
	"github.com/kxg3030/shermie-proxy/Core"
	"github.com/kxg3030/shermie-proxy/Log"
	"net"
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
	proxy := flag.String("proxy", "", "proxy remote host")
	to := flag.String("to", "", "tcp remote host")
	network := flag.String("network", "", "force interface address")
	flag.Parse()
	if *port == "0" {
		Log.Log.Fatal("port required")
		return
	}
	// 解析端口
	portPair := strings.Split(*port, ",")
	// 解析网卡
	networkPair := strings.Split(*network, ",")
	if len(portPair) != len(networkPair) {
		Log.Log.Fatal("代理端口数量和网卡数量必须一致")
	}
	for key, _ := range portPair {
		go ListenBranch(portPair[key], *nagle, *proxy, *to, networkPair[key])
	}
	select {}
}

func ListenBranch(port string, nagle bool, proxy string, to string, network string) {
	// 启动服务
	s := Core.NewProxyServer(port, nagle, proxy, to, network)

	// 注册tcp连接事件
	s.OnTcpConnectEvent = func(conn net.Conn) {

	}
	// 注册tcp关闭事件
	s.OnTcpCloseEvent = func(conn net.Conn) {

	}

	s.OnHttpRequestEvent = func(message []byte, request *http.Request, resolve Core.ResolveHttpRequest, conn net.Conn) bool {
		Log.Log.Println("HttpRequestEvent：" + conn.RemoteAddr().String())
		// 可以在这里做数据修改
		resolve(message, request)
		// 如果正常处理必须返回true，如果不需要发送请求，返回false，一般在自己操作conn的时候才会用到
		return true
	}
	// 注册http服务器响应事件函数
	s.OnHttpResponseEvent = func(body []byte, response *http.Response, resolve Core.ResolveHttpResponse, conn net.Conn) bool {
		mimeType := response.Header.Get("Content-Type")
		if strings.Contains(mimeType, "json") {
			Log.Log.Println("HttpResponseEvent：" + string(body))
		}
		// 可以在这里做数据修改
		resolve(body, response)
		// 如果正常处理必须返回true，如果不需要将数据返回给客户端，返回false，一般在自己操作conn的时候才会用到
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
