package Core

import (
	"bufio"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Contract"
	"github.com/kxg3030/shermie-proxy/Log"
	"github.com/viki-org/dnscache"
	"net"
	"net/http"
	"time"
)

type HttpRequestEvent func(message []byte, request *http.Request, resolve ResolveHttpRequest)
type HttpResponseEvent func(message []byte, response *http.Response, resolve ResolveHttpResponse)

type Socket5ResponseEvent func(message []byte, resolve ResolveSocks5) (int, error)
type Socket5RequestEvent func(message []byte, resolve ResolveSocks5) (int, error)

type WsRequestEvent func(msgType int, message []byte, resolve ResolveWs) error
type WsResponseEvent func(msgType int, message []byte, resolve ResolveWs) error

type TcpServerStreamEvent func(message []byte, resolve ResolveTcp) (int, error)
type TcpClientStreamEvent func(message []byte, resolve ResolveTcp) (int, error)

const (
	MethodGet     = 0x47
	MethodConnect = 0x43
	MethodPost    = 0x50
	MethodPut     = 0x50
	MethodDelete  = 0x44
	MethodOptions = 0x4F
	MethodHead    = 0x48

	SocksFive = 0x5
)

type ProxyServer struct {
	nagle                  bool
	to                     string
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

func NewProxyServer(port string, nagle bool, proxy string, to string) *ProxyServer {
	return &ProxyServer{
		port:  port,
		dns:   dnscache.New(time.Minute * 5),
		nagle: nagle,
		proxy: proxy,
		to:    to,
	}
}

func (i *ProxyServer) Start() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%s", i.port))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
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
	Log.Log.Println("代理监听端口:0.0.0.0:" + i.port)
	Log.Log.Println("请先手动将根证书.crt文件导入到系统中,可以在根目录或访问http://shermie-proxy.io/tls获取证书文件")
}

func (i *ProxyServer) MultiListen() {
	for s := 0; s < 5; s++ {
		go func() {
			for {
				conn, err := i.listener.Accept()
				if err != nil {
					if e, ok := err.(net.Error); ok && e.Timeout() {
						Log.Log.Println("接受连接超时：" + err.Error())
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
	// 预读取一段字节,https、ws、wss读取到的数据为：CONNECT xx.com:8080 HTTP/1.1
	peek, err := reader.Peek(3)
	if err != nil {
		return
	}
	peer := ConnPeer{server: i, conn: conn, writer: writer, reader: reader}
	switch peek[0] {
	case MethodGet, MethodPost, MethodDelete, MethodOptions, MethodHead, MethodConnect:
		process = &ProxyHttp{ConnPeer: peer}
		break
	case SocksFive:
		process = &ProxySocks5{ConnPeer: peer}
	default:
		process = &ProxyTcp{ConnPeer: peer}
	}
	process.Handle()
}
