package Core

import (
	"github.com/kxg3030/shermie-proxy/Log"
	"io"
	"net"
)

type ProxyTcp struct {
	ConnPeer
	target net.Conn
	port   string
}

func (i *ProxyTcp) Handle() {
	defer func() {
		i.ConnPeer.conn.Close()
	}()
	tcpAddr, err := net.ResolveTCPAddr("tcp", i.server.proxy)
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
	if !i.server.nagle {
		_ = conn.SetNoDelay(false)
	}
	stop := make(chan error, 2)
	go func() {
		_, err := io.Copy(i.ConnPeer.conn, conn)
		if err != nil {
			stop <- err
		}
	}()
	go func() {
		_, err := io.Copy(conn, i.ConnPeer.conn)
		if err != nil {
			stop <- err
		}
	}()
	err = <-stop
	Log.Log.Println("转发tcp数据错误：" + err.Error())
}
