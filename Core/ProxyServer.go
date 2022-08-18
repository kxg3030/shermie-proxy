package Core

import (
	"bufio"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Contract"
	"github.com/kxg3030/shermie-proxy/Core/Websocket"
	"github.com/kxg3030/shermie-proxy/Log"
	"github.com/viki-org/dnscache"
	"net"
	"net/http"
	"time"
)

type HttpRequestEvent func(request *http.Request)
type HttpResponseEvent func(response *http.Response)

type Socket5ResponseEvent func(message []byte)
type Socket5RequestEvent func(message []byte)

type WsRequestEvent func(msgType int, message []byte, clientConn *Websocket.Conn, resolve ResolveWs) error
type WsResponseEvent func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve ResolveWs) error

type TcpServerStreamEvent func(message []byte)
type TcpClientStreamEvent func(message []byte)

type ProxyServer struct {
	nagle                  bool
	proxy                  string
	port                   string
	listener               *net.TCPListener
	dns                    *dnscache.Resolver
	OnHttpRequestEvent     HttpRequestEvent
	OnHttpResponseEvent    HttpResponseEvent
	OnWsRequestEvent       WsRequestEvent
	OnWsResponseEvent      WsResponseEvent
	OnSocket5ResponseEvent Socket5ResponseEvent
	OnSocket5RequestEvent  Socket5RequestEvent
	OnTcpServerStreamEvent TcpServerStreamEvent
	OnTcpClientStreamEvent TcpClientStreamEvent
}

func NewProxyServer(port string, nagle bool, proxy string) *ProxyServer {
	return &ProxyServer{
		port:  port,
		dns:   dnscache.New(time.Minute * 5),
		nagle: nagle,
		proxy: proxy,
	}
}

func (i *ProxyServer) Start() error {
	// 解析地址
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%s", i.port))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// 监听服务
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	i.listener = listener
	i.Logo()
	i.MultiListen()
	select {}
}

func (i *ProxyServer) Logo() {
	logo := ` 
 ______     __  __     ______     ______     __    __     __     ______                   ______   ______     ______     __  __     __  __ 
/\  ___\   /\ \_\ \   /\  ___\   /\  == \   /\ "-./  \   /\ \   /\  ___\                 /\  == \ /\  == \   /\  __ \   /\_\_\_\   /\ \_\ \
\ \___  \  \ \  __ \  \ \  __\   \ \  __<   \ \ \-./\ \  \ \ \  \ \  __\    ┌--------┐   \ \  _-/ \ \  __<   \ \ \/\ \  \/_/\_\/_  \ \____ \ 
 \/\_____\  \ \_\ \_\  \ \_____\  \ \_\ \_\  \ \_\ \ \_\  \ \_\  \ \_____\  └--------┘    \ \_\    \ \_\ \_\  \ \_____\   /\_\/\_\  \/\_____\
  \/_____/   \/_/\/_/   \/_____/   \/_/ /_/   \/_/  \/_/   \/_/   \/_____/                 \/_/     \/_/ /_/   \/_____/   \/_/\/_/   \/_____/ 
`
	Log.Log.Println(logo)
	Log.Log.Println("代理监听端口:0.0.0.0:" + i.port + "┏如果是新生成的证书文件，请先手动将根证书.crt文件导入到系统中)")
}

func (i *ProxyServer) MultiListen() {
	for s := 0; s < 5; s++ {
		go func() {
			for {
				conn, err := i.listener.Accept()
				if err != nil {
					if e, ok := err.(net.Error); ok && e.Temporary() {
						Log.Log.Println("接受连接失败,重试：" + err.Error())
						time.Sleep(time.Second / 20)
					} else {
						Log.Log.Println("接受连接失败：" + err.Error())
					}
					continue
				}
				go i.handle(conn)
			}
		}()
	}
}

func (i *ProxyServer) handle(conn net.Conn) {
	var process Contract.IServerProcesser
	defer conn.Close()
	// 使用bufio读取,原conn的句柄数据被读完
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	// 预读取一段字节,https、ws、wss读取到的数据为：CONNECT wan.xx.com:8080 HTTP/1.1
	peek, err := reader.Peek(1)
	if err != nil {
		return
	}
	peekHex := fmt.Sprintf("0x%x", peek[0])
	peer := ConnPeer{server: i, conn: conn, writer: writer, reader: reader,}
	switch peekHex {
	case "0x47", "0x43", "0x50", "0x4f", "0x44", "0x48":
		process = &ProxyHttp{ConnPeer: peer}
		break
	case "0x5":
		process = &ProxySocket{ConnPeer: peer}
	default:
		process = &ProxyTcp{ConnPeer: peer}
	}
	process.Handle()
}
