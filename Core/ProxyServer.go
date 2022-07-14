package Core

import (
	"bufio"
	"fmt"
	"github/shermie-proxy/Log"
	"net"
	"net/http"
)

var HttpHeadMap = map[string]int{
	// CONNECT
	"0x47": 0x47,
	// GET
	"0x43": 0x43,
	// POST
	"0x50": 0x50,
}

type ProxyServer struct {
	port            string
	listener        *net.TCPListener
	OnRequestEvent  func(request *http.Request)
	OnResponseEvent func(response *http.Response)
	OnReceiveEvent  error
	OnSendEvent     error
	OnPacketEvent   func(msgType int, message []byte)
	OnSendToEvent   func(msgType int, message []byte)
}

func NewProxyServer(port string) *ProxyServer {
	p := &ProxyServer{
		port: port,
	}
	return p
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
	Log.Log.Println("服务监听端口：" + i.port + "(如果是新生成的证书文件，请先手动将根证书.crt文件导入到系统中——by.失色天空)")
	i.MultiListen()
	select {}
}

func (i *ProxyServer) MultiListen() {
	for s := 0; s < 10; s++ {
		go func() {
			for {
				conn, err := i.listener.Accept()
				if err != nil {
					Log.Log.Println("接受连接失败：" + err.Error())
					continue
				}
				go i.handle(conn)
			}
		}()
	}
}

func (i *ProxyServer) handle(conn net.Conn) {
	defer conn.Close()
	// 使用bufio读取,原conn的句柄数据被读完(无法再次读取),保存到bufio缓冲区中,只有缓冲区能重复读取
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	// 预读取一段字节,https、ws、wss读取到的数据为：CONNECT wan.xx.com:8080 HTTP/1.1
	peek, err := reader.Peek(1)
	if err != nil {
		return
	}
	peekHex := fmt.Sprintf("0x%x", peek[0])
	if peekHex == "0x5" {
		proxySocket := NewProxySocket()
		proxySocket.reader = reader
		proxySocket.writer = writer
		proxySocket.conn = conn
		proxySocket.server = i
		proxySocket.handle()
		return
	}
	if _, ok := HttpHeadMap[peekHex]; ok {
		proxyHttp := NewProxyHttp()
		proxyHttp.reader = reader
		proxyHttp.writer = writer
		proxyHttp.conn = conn
		proxyHttp.server = i
		proxyHttp.handle()
	}
}
