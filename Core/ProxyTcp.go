package Core

import (
	"crypto/tls"
	"errors"
	"io"
	"net"

	"github.com/kxg3030/shermie-proxy/Log"
)

const TcpServer = "server"
const TcpClient = "client"

type ProxyTcp struct {
	ConnPeer
	target net.Conn
	port   string
}

type ResolveTcp func(buff []byte) (int, error)

func (i *ProxyTcp) Handle() {
	var tcpAddr *net.TCPAddr
	var err error
	tcpAddr, err = net.ResolveTCPAddr("tcp", i.server.to)
	if err != nil {
		Log.Log.Println("解析tcp代理目标地址错误：" + err.Error())
		return
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	defer func() {
		_ = conn.Close()
	}()
	if err != nil {
		Log.Log.Println("连接tcp代理目标地址错误：" + err.Error())
		return
	}
	// 处理tls握手
	host, port, _ := net.SplitHostPort(conn.RemoteAddr().String())
	certificate, err := Cache.GetCertificate(host, port)
	if err != nil {
		Log.Log.Println(conn.RemoteAddr().String() + "：获取证书失败：" + err.Error())
		return
	}
	if _, ok := certificate.(tls.Certificate); !ok {
		return
	}
	cert := certificate.(tls.Certificate)
	sslConn := tls.Server(i.conn, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	// tls校验
	err = sslConn.Handshake()
	if err == nil {
		i.ConnPeer.conn = sslConn
	}
	if !i.server.nagle {
		_ = conn.SetNoDelay(false)
	}
	stop := make(chan error, 2)
	go i.Transport(stop, i.ConnPeer.conn, conn, TcpClient)
	go i.Transport(stop, conn, i.ConnPeer.conn, TcpServer)
	err = <-stop
	Log.Log.Println("转发tcp数据错误：" + err.Error())
}

func (i *ProxyTcp) Transport(out chan<- error, originConn net.Conn, targetConn net.Conn, role string) {
	buff := make([]byte, 4096)
	resolve := ResolveTcp(func(buff []byte) (int, error) {
		return targetConn.Write(buff)
	})
	var writeLen int
	for {
		readLen, err := originConn.Read(buff)
		if readLen > 0 {
			buff = buff[0:readLen]
			if role == TcpServer {
				if i.server.OnTcpServerStreamEvent != nil {
					writeLen, err = i.server.OnTcpServerStreamEvent(buff, resolve, i.conn)
				} else {
					writeLen, err = resolve(buff)
				}
			} else {
				if i.server.OnTcpClientStreamEvent != nil {
					writeLen, err = i.server.OnTcpClientStreamEvent(buff, resolve, i.conn)
				} else {
					writeLen, err = resolve(buff)
				}
			}
			if writeLen < 0 || readLen < writeLen {
				writeLen = 0
				if err == nil {
					out <- errors.New("tcp代理写入目标服务器错误-1")
					break
				}
			}
			if readLen != writeLen {
				out <- errors.New("tcp代理写入目标服务器错误-2")
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				out <- errors.New("tcp代理读取客户端数据错误-1")
			}
			break
		}
		buff = buff[:]
	}
}
