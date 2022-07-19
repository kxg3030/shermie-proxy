package Core

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"github/kxg3030/shermie-proxy/Log"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type ProxySocket struct {
	server   *ProxyServer
	writer   *bufio.Writer
	reader   *bufio.Reader
	request  *http.Request
	response *http.Response
	target   net.Conn
	conn     net.Conn
	port     string
}

const (
	// 预留位
	Rsv = 0x00
	// 命令
	CommandConn = 0x01
	CommandBind = 0x02
	CommandUdp  = 0x03
	// 目标类型
	TargetIpv4   = 0x01
	TargetIpv6   = 0x04
	TargetDomain = 0x03
	Version      = 0x5
)

type CustomWriter struct {
}

func (i *CustomWriter) Write(b []byte) (int, error) {
	buff := bytes.NewBuffer(b)
	Log.Log.Println(buff)
	return len(b), nil
}

func (i *CustomWriter) Read(b []byte) (int, error) {
	buff := bytes.NewBuffer(b)
	Log.Log.Println(buff)
	return len(b), nil
}

func NewCustomWriter() *CustomWriter {
	return &CustomWriter{}
}

func NewProxySocket() *ProxySocket {
	return &ProxySocket{}
}

func (i *ProxySocket) handle() {
	// 读取版本号
	version, err := i.reader.ReadByte()
	if err != nil {
		Log.Log.Println("读取socket5版本号错误：" + err.Error())
		return
	}
	if version != Version {
		Log.Log.Println("socket5版本号不匹配")
		return
	}
	// 读取支持的方法
	methodNum, err := i.reader.ReadByte()
	if err != nil {
		Log.Log.Println("读取socket5支持方法数量错误：" + err.Error())
		return
	}
	if methodNum < 0 || methodNum > 0xFF {
		Log.Log.Println("socket5支持方法参数错误")
		return
	}
	// 是否需要账号密码验证
	var requiredAuth bool
	method := uint8(0x00)
	// 读取所有的方法列表
	for n := 0; n < int(methodNum); n++ {
		method, err = i.reader.ReadByte()
		if err != nil {
			Log.Log.Println("读取socket5支持错误：" + err.Error())
			return
		}
		if method == 0x02 {
			requiredAuth = true
		}
	}

	_, err = i.writer.Write([]byte{version, method})
	if err != nil {
		Log.Log.Println("返回数据错误：" + err.Error())
		return
	}
	_ = i.writer.Flush()
	if requiredAuth {
		// TODO 账号密码验证
		return
	}
	// 读取版本号
	version, err = i.reader.ReadByte()
	if version != Version {
		Log.Log.Println("socket5版本号错误")
		return
	}
	// 读取命令
	command, err := i.reader.ReadByte()
	if err != nil {
		Log.Log.Println("读取socket5命令错误")
		return
	}
	if command != CommandConn && command != CommandBind && command != CommandUdp {
		Log.Log.Println("不支持socket5命令")
		return
	}
	// 读取保留位
	rsv, err := i.reader.ReadByte()
	if err != nil || rsv != Rsv {
		Log.Log.Println("读取socket5保留位错误")
		return
	}
	// 读取目标地址类型
	targetType, err := i.reader.ReadByte()
	if err != nil {
		Log.Log.Println("读取socket5保留位错误")
		return
	}
	if targetType != TargetIpv4 && targetType != TargetIpv6 && targetType != TargetDomain {
		Log.Log.Println("不支持socket5地址")
		return
	}
	var hostname string
	switch targetType {
	case TargetIpv4:
		buffer := make([]byte, 4)
		// 读4字节
		n, err := i.reader.Read(buffer)
		if err != nil || n != len(buffer) {
			Log.Log.Println("读取ipv4地址错误")
			return
		}
		hostname = net.IP(buffer).String()
		break
	case TargetIpv6:
		buffer := make([]byte, 16)
		// 读16字节
		n, err := i.reader.Read(buffer)
		if err != nil || n != len(buffer) {
			Log.Log.Println("读取ipv6地址错误")
			return
		}
		hostname = net.IP(buffer).String()
		break
	case TargetDomain:
		// 读取域名长度
		domainLen, err := i.reader.ReadByte()
		if err != nil || domainLen <= 0 {
			Log.Log.Println("读取域名地址错误")
			return
		}
		buffer := make([]byte, domainLen)
		n, err := i.reader.Read(buffer)
		if err != nil || n != len(buffer) {
			Log.Log.Println("读取域名地址错误")
			return
		}
		addr, err := net.ResolveIPAddr("ip", string(buffer))
		if err != nil {
			Log.Log.Println("读取域名地址错误：" + err.Error())
			hostname = string(buffer)
		} else {
			hostname = addr.String()
		}
		break
	}
	// 读端口号,大端
	buffer := make([]byte, 2)
	_, err = i.reader.Read(buffer)
	if err != nil {
		Log.Log.Println("读取端口号错误：" + err.Error())
		return
	}
	i.port = strconv.Itoa(int(i.ByteToInt(buffer)))
	hostname = fmt.Sprintf("%s:%s", hostname, i.port)
	// 写入版本号
	_ = i.writer.WriteByte(Version)
	if command == 0x03 {
		i.target, err = net.Dial("udp", hostname)
	} else {
		if i.port == "443" {
			i.target, err = tls.Dial("tcp", hostname, &tls.Config{
				InsecureSkipVerify: false,
			})
		} else {
			i.target, err = net.Dial("tcp", hostname)
		}
	}
	Log.Log.Println("待连接的目标服务器：" + hostname)
	// 写入Rep
	if err != nil {
		Log.Log.Println("连接目标服务器失败：" + hostname + " " + err.Error())
		_ = i.writer.WriteByte(0x01)
		_ = i.writer.Flush()
		return
	} else {
		_ = i.writer.WriteByte(0x00)
	}
	// 写入Rsv
	_ = i.writer.WriteByte(Rsv)
	remoteAddr := i.target.RemoteAddr().String()
	host, _, _ := net.SplitHostPort(remoteAddr)
	if i.IpV4(host) {
		_ = i.writer.WriteByte(TargetIpv4)
		_, _ = i.writer.Write(net.ParseIP(host).To4())
	}
	if i.IpV6(host) {
		_ = i.writer.WriteByte(TargetIpv6)
		_, _ = i.writer.Write(net.ParseIP(host).To16())
	}
	if !i.IpV4(host) && !i.IpV6(host) {
		_ = i.writer.WriteByte(TargetDomain)
		_ = i.writer.WriteByte(byte(len(hostname)))
		_, _ = i.writer.WriteString(hostname)
	}
	// 写入端口
	_, _ = i.writer.Write(buffer)
	err = i.writer.Flush()
	if err != nil {
		Log.Log.Println("写入socket5握手错误：" + err.Error())
		return
	}
	out := make(chan error, 2)
	if command == 0x01 {
		go i.Transport(out, i.conn, i.target, "tcp client to server")
		go i.Transport(out, i.target, i.conn, "tcp server to client")
		<-out
	}
}

func (i *ProxySocket) Transport(out chan<- error, originConn net.Conn, targetConn net.Conn, role string) {
	for {
		originReader := io.MultiReader(originConn, NewCustomWriter())
		targetWriter := io.MultiWriter(targetConn, NewCustomWriter())
		_, err := io.Copy(targetWriter, originReader)
		if err != nil {
			out <- err
		}
	}
}

func (i *ProxySocket) IpV4(ipAddr string) bool {
	ip := net.ParseIP(ipAddr)
	return ip != nil && strings.Contains(ipAddr, ".")
}

func (i *ProxySocket) IpV6(ipAddr string) bool {
	ip := net.ParseIP(ipAddr)
	return ip != nil && strings.Contains(ipAddr, ":")
}

// 字节转整型
func (i *ProxySocket) ByteToInt(input []byte) int32 {
	return int32(input[0]&0xFF)<<8 | int32(input[1]&0xFF)
}
