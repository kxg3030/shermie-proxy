package Core

import "net"

type ProxyTcp struct {
	ConnPeer
	target net.Conn
	port   string
}

func (i *ProxyTcp) Handle() {
	i.conn.Close()
}
