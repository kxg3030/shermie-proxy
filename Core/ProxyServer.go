package Core

import (
	"bufio"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Contract"
	"github.com/kxg3030/shermie-proxy/Log"
	"github.com/kxg3030/shermie-proxy/Utils"
	"github.com/viki-org/dnscache"
	"net"
	"net/http"
	"runtime"
	"time"
)

type HttpRequestEvent func(message []byte, request *http.Request, resolve ResolveHttpRequest, conn net.Conn)
type HttpResponseEvent func(message []byte, response *http.Response, resolve ResolveHttpResponse, conn net.Conn)

type Socks5ResponseEvent func(message []byte, resolve ResolveSocks5, conn net.Conn) (int, error)
type Socks5RequestEvent func(message []byte, resolve ResolveSocks5, conn net.Conn) (int, error)

type WsRequestEvent func(msgType int, message []byte, resolve ResolveWs, conn net.Conn) error
type WsResponseEvent func(msgType int, message []byte, resolve ResolveWs, conn net.Conn) error

type TcpConnectEvent func(conn net.Conn)
type TcpClosetEvent func(conn net.Conn)
type TcpServerStreamEvent func(message []byte, resolve ResolveTcp, conn net.Conn) (int, error)
type TcpClientStreamEvent func(message []byte, resolve ResolveTcp, conn net.Conn) (int, error)

const (
	MethodGet     = 0x47
	MethodConnect = 0x43
	MethodPost    = 0x50
	MethodPut     = 0x50
	MethodDelete  = 0x44
	MethodOptions = 0x4F
	MethodHead    = 0x48
	SocksFive     = 0x5
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
	OnSocks5ResponseEvent  Socks5ResponseEvent
	OnSocks5RequestEvent   Socks5RequestEvent
	OnTcpConnectEvent      TcpConnectEvent
	OnTcpCloseEvent        TcpClosetEvent
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

func (i *ProxyServer) Install() {
	if runtime.GOOS == "windows" {
		err := Utils.InstallCert("cert.crt")
		if err != nil {
			Log.Log.Println(err.Error())
			return
		}
		Log.Log.Println("已安装系统证书")
		err = Utils.SetWindowsProxy(fmt.Sprintf("127.0.0.1:%s", i.port))
		if err != nil {
			Log.Log.Println(err.Error())
			return
		}
		Log.Log.Println("已设置系统代理")
		return
	}
	Log.Log.Println("非windows系统请手动安装证书并设置代理,可以在根目录或访问http://shermie-proxy.io/tls获取证书文件")
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
	i.Install()
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
	Log.Log.Println("0.0.0.0:" + i.port)
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
	defer func() {
		i.OnTcpCloseEvent(conn)
		conn.Close()
	}()
	i.OnTcpConnectEvent(conn)
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	// https、ws、wss读取到的数据为：CONNECT xx.com:8080 HTTP/1.1
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
