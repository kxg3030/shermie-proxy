package Core

import (
	"bufio"
	"net"
)

type ConnPeer struct {
	conn   net.Conn
	writer *bufio.Writer
	reader *bufio.Reader
	server *ProxyServer
}
