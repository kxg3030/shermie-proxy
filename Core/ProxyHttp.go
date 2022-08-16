package Core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Core/Websocket"
	"github.com/kxg3030/shermie-proxy/Log"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const ConnectSuccess = "HTTP/1.1 200 Connection Established\r\n\r\n"
const ConnectFailed = "HTTP/1.1 502 Bad Gateway\r\n\r\n"
const SslFileHost = "zt.io"

type ProxyHttp struct {
	ConnPeer
	request  *http.Request
	response *http.Response
	upgrade  *Websocket.Upgrader
	target   net.Conn
	ssl      bool
	port     string
}

type ResolveWs func(msgType int, message []byte, wsConn *Websocket.Conn) error

func NewProxyHttp() *ProxyHttp {
	p := &ProxyHttp{
	}
	return p
}

// tcp连接处理入口
func (i *ProxyHttp) Handle() {
	request, err := http.ReadRequest(i.reader)
	if err != nil {
		return
	}
	i.port = "-1"
	if hostname := strings.Split(request.Host, ":"); len(hostname) > 1 {
		i.port = hostname[len(hostname)-1]
	}
	i.request = request
	// 如果是connect方法则是https请求或者ws、wss请求
	if i.request.Method == http.MethodConnect {
		i.ssl = true
		i.handleSslRequest()
		return
	}
	// 否则是普通请求
	i.ssl = false
	i.handleRequest()
}

// 处理请求入口
func (i *ProxyHttp) handleRequest() {
	var err error
	if i.request.URL == nil {
		Log.Log.Println("请求地址为空")
		return
	}
	// 如果是下载证书,返回证书
	if i.request.Host == SslFileHost && i.request.URL.Path == "/ssl" {
		response := http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/x-x509-ca-cert"},
			},
			Body: io.NopCloser(bytes.NewReader(Cert.RootCaStr)),
		}
		_ = response.Write(i.conn)
		return
	}
	body, _ := i.ReadBody(i.request.Body)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnRequestEvent(i.request)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	// 处理正常请求,获取响应
	i.response, err = i.Transport(i.request)
	if i.response == nil {
		Log.Log.Println("远程服务器无响应")
		return
	}
	defer func() {
		if i.response.Body != nil {
			i.response.Body.Close()
		}
	}()
	if err != nil {
		Log.Log.Println("获取远程服务器响应失败：" + err.Error())
		return
	}
	body, _ = i.ReadBody(i.response.Body)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnResponseEvent(i.response)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	defer func() {
		if i.response.Body != nil {
			i.response.Body.Close()
		}
	}()
	_ = i.response.Write(i.conn)
}

// 读取http请求体
func (i *ProxyHttp) ReadBody(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return []byte{}, nil
	}
	body, err := io.ReadAll(reader)
	return body, err
}

// 移除请求头
func (i *ProxyHttp) RemoveHeader(header http.Header) {
	removeHeaders := []string{
		"Keep-Alive",
		"Transfer-Encoding",
		"TE",
		"Connection",
		"Trailer",
		"Upgrade",
		"Proxy-Authorization",
		"Proxy-Authenticate",
		"Connection",
		"Accept-Encoding",
	}
	for _, value := range removeHeaders {
		if v := header.Get(value); len(v) > 0 {
			if strings.EqualFold(value, "Connection") {
				for _, customerHeader := range strings.Split(value, ",") {
					header.Del(strings.Trim(customerHeader, " "))
				}
			}
			header.Del(value)
		}
	}
}

// http请求转发
func (i *ProxyHttp) Transport(request *http.Request) (*http.Response, error) {
	// 去除一些头部
	i.RemoveHeader(request.Header)
	response, err := (&http.Transport{
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 60 * time.Second,
		DialContext:           i.DialContext(),
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}).RoundTrip(request)
	if err != nil {
		return nil, err
	}
	// 去除一些头部
	i.RemoveHeader(response.Header)
	return response, err
}

// 处理tls请求
func (i *ProxyHttp) handleSslRequest() {
	var err error
	if i.port == "443" {
		i.target, err = tls.Dial("tcp", i.request.Host, &tls.Config{
			InsecureSkipVerify: true,
		})
	} else {
		i.target, err = net.Dial("tcp", i.request.Host)
	}
	if err != nil {
		_, err = i.conn.Write([]byte(ConnectFailed))
		return
	}
	_ = i.target.Close()
	// 向源连接返回连接成功
	_, err = i.conn.Write([]byte(ConnectSuccess))
	if err != nil {
		Log.Log.Println("返回连接状态失败：" + err.Error())
		return
	}
	// 建立ssl连接并返回给源
	i.SslReceiveSend()
}

// 设置请求头
func (i *ProxyHttp) SetRequest(request *http.Request) *http.Request {
	request.Header.Set("Connection", "false")
	request.URL.Host = request.Host
	request.URL.Scheme = "https"
	return request
}

// tls数据接收发送
func (i *ProxyHttp) SslReceiveSend() {
	var err error
	certificate, err := Cache.GetCertificate(i.request.Host, i.port)
	if err != nil {
		Log.Log.Println(i.request.Host + "：获取证书失败：" + err.Error())
		return
	}
	if _, ok := certificate.(tls.Certificate); !ok {
		return
	}
	cert := certificate.(tls.Certificate)
	sslConn := tls.Server(i.conn, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	// ssl校验
	err = sslConn.Handshake()
	// 如果不是http的ssl请求,则说明是普通ws请求(ws请求会ssl校验报错),这里专门处理这种情况
	if err != nil {
		i.ssl = false
		if err == io.EOF || strings.Index(err.Error(), "closed") != -1 {
			Log.Log.Println("客户端连接超时：" + err.Error())
			return
		}
		i.handleWsShakehandErr(sslConn.ReadLastTimeBytes())
		return
	}
	_ = sslConn.SetDeadline(time.Now().Add(time.Second * 60))
	defer func() {
		_ = sslConn.Close()
	}()
	i.conn = sslConn
	i.ssl = true
	i.reader = bufio.NewReader(i.conn)
	i.request, err = http.ReadRequest(i.reader)
	if err != nil {
		if err == io.EOF {
			Log.Log.Println("浏览器ssl连接断开：" + err.Error())
			return
		}
		Log.Log.Println("读取ssl连接请求数据失败：" + err.Error())
		return
	}
	// 如果包含upgrade同步说明是wss请求
	if i.request.Header.Get("Connection") == "Upgrade" {
		i.handleWssRequest()
		return
	}
	i.request = i.SetRequest(i.request)
	body, _ := i.ReadBody(i.request.Body)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnRequestEvent(i.request)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.response, err = i.Transport(i.request)
	if err != nil {
		Log.Log.Println("远程服务器响应失败：" + err.Error())
		return
	}
	if i.response == nil {
		Log.Log.Println("远程服务器无响应")
		return
	}
	defer func() {
		if i.response.Body != nil {
			_ = i.response.Body.Close()
		}
	}()
	body, _ = i.ReadBody(i.response.Body)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnResponseEvent(i.response)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	// 如果写入的数据比返回的头部指定长度还长,就会报错,这里手动计算返回的数据长度
	i.response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	err = i.response.Write(i.conn)
	if err != nil {
		if strings.Contains(err.Error(), "aborted") {
			Log.Log.Println("代理返回响应数据失败：连接已关闭")
			return
		}
		Log.Log.Println("代理返回响应数据失败：" + err.Error())
	}
}

// 加密wss请求
func (i *ProxyHttp) handleWssRequest() {
	i.handleWsRequest()
}

// 普通ws请求
func (i *ProxyHttp) handleWsShakehandErr(rawProtolInput []byte) {
	var err error
	// 获取浏览器发送给服务器的头部和数据,构建一个完整的请求对象
	rawInput := string(rawProtolInput)
	_ = i.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	rawInputList := strings.Split(rawInput, "\r\n")
	// 读取请求方法
	wsMethodList := strings.Split(rawInputList[0], " ")
	// 构建请求
	wsRequest := &http.Request{
		Method: wsMethodList[0],
		Header: map[string][]string{},
	}
	for _, value := range rawInputList {
		// 填充header
		headerKeValList := strings.Split(value, ": ")
		if len(headerKeValList) <= 1 {
			continue
		}
		wsRequest.Header.Set(headerKeValList[0], headerKeValList[1])
		// 填充host
		if headerKeValList[0] == "Host" {
			wsRequest.Host = headerKeValList[1]
			wsRequest.RequestURI = fmt.Sprintf("http://%s", wsRequest.Host)
			wsRequest.URL, err = url.Parse(fmt.Sprintf("%s%s", wsRequest.RequestURI, wsMethodList[1]))
			if err != nil {
				Log.Log.Println("解析ws请求地址错误：" + err.Error())
				return
			}
		}
		// 填充content-length
		if headerKeValList[0] == "Content-Length" {
			contentLen, _ := strconv.Atoi(headerKeValList[1])
			contentHeaderLen := len(rawInput)
			bodyLen := contentHeaderLen - contentLen
			wsRequest.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(rawInput[bodyLen:])))
			wsRequest.ContentLength = int64(bodyLen)
		}
	}
	i.request = wsRequest
	i.handleWsRequest()
}

// 处理ws请求
func (i *ProxyHttp) handleWsRequest() bool {
	if i.request.Header.Get("Upgrade") == "" {
		return false
	}
	if i.upgrade == nil {
		i.upgrade = &Websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
	}
	// 如果有携带自定义参数,添加上
	i.upgrade.Subprotocols = []string{i.request.Header.Get("Sec-WebSocket-Protocol")}
	recorder := httptest.NewRecorder()
	// 开始ws校验,向浏览器返回Sec-WebSocket-Accept头,ws握手完成
	clientWsConn, err := i.upgrade.Upgrade(recorder, i.request, nil, i.conn, bufio.NewReadWriter(i.reader, i.writer))
	if err != nil {
		Log.Log.Println("升级ws协议失败：" + err.Error())
		return true
	}
	defer func() {
		_ = clientWsConn.Close()
	}()
	hostname := fmt.Sprintf("%s://%s%s", func() string {
		if i.ssl {
			return "wss"
		}
		return "ws"
	}(), i.request.Host, i.request.URL.Path)
	if i.request.URL.RawQuery != "" {
		hostname += "?" + i.request.URL.RawQuery
	}
	// 去掉ws的头部,因为后续工具类会自己生成并附加到请求中
	i.RemoveWsHeader()
	var dialer Websocket.Dialer
	dialer = Websocket.Dialer{}
	// 如果是wss,客户端传输层忽略证书校验
	if i.ssl {
		dialer = Websocket.Dialer{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			HandshakeTimeout: time.Second * 10,
		}
	}
	dialer.NetDialContext = i.DialContext()
	targetWsConn, _, err := dialer.Dial(hostname, i.request.Header)
	if err != nil {
		Log.Log.Println("连接ws服务器失败：" + err.Error())
		return true
	}
	defer func() {
		_ = targetWsConn.Close()
	}()
	stop := make(chan error, 2)
	// 读取浏览器数据(长连接)
	go func() {
		defer func() {
			_ = targetWsConn.Close()
		}()
		for {
			msgType, message, err := targetWsConn.ReadMessage()
			if err != nil {
				if Websocket.IsUnexpectedCloseError(err, Websocket.CloseGoingAway, Websocket.CloseAbnormalClosure) {
					stop <- fmt.Errorf("读取wss服务器数据失败-1：%w", err)
					break
				}
				stop <- fmt.Errorf("读取wss服务器数据失败-2：%w", err)
				break
			}
			err = i.server.OnServerPacketEvent(msgType, message, clientWsConn, func(msgType int, message []byte, wsConn *Websocket.Conn) error {
				return wsConn.WriteMessage(msgType, message)
			})
			if err != nil {
				stop <- fmt.Errorf("发送wss浏览器数据失败-1：%w", err)
			}
		}
	}()
	go func() {
		defer func() {
			_ = clientWsConn.Close()
		}()
		for {
			msgType, message, err := clientWsConn.ReadMessage()
			if err != nil {
				if Websocket.IsUnexpectedCloseError(err, Websocket.CloseGoingAway, Websocket.CloseAbnormalClosure) {
					stop <- fmt.Errorf("读取wss浏览器数据失败-1：%w", err)
					break
				}
				stop <- fmt.Errorf("读取wss浏览器数据失败-2：%w", err)
				break
			}
			err = i.server.OnClientPacketEvent(msgType, message, targetWsConn, func(msgType int, message []byte, wsConn *Websocket.Conn) error {
				return wsConn.WriteMessage(msgType, message)
			})
			if err != nil {
				stop <- fmt.Errorf("发送wss浏览器数据失败-1：%w", err)
			}
		}
	}()
	err = <-stop
	Log.Log.Println(err.Error())
	return false
}

func (i *ProxyHttp) DialContext() func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	return func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		separator := strings.LastIndex(addr, ":")
		ipList, err := i.server.dns.Fetch(addr[:separator])
		var ip string
		for _, item := range ipList {
			ip = item.String()
			if !strings.Contains(ip, ":") {
				break
			}
		}
		tcpAddr, _ := net.ResolveTCPAddr("tcp", ip+addr[separator:])
		conn, err = net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			return conn, err
		}
		// 是否关闭nagle算法
		tcpConn, ok := conn.(*net.TCPConn)
		if ok {
			_ = tcpConn.SetNoDelay(i.server.nagle)
		}
		return tcpConn, err
	}
}

// 连接是否可用
func (i *ProxyHttp) WsIsConnected(conn *Websocket.Conn) bool {
	err := conn.WriteMessage(1, nil)
	return err == nil
}

// 移除ws请求头
func (i *ProxyHttp) RemoveWsHeader() {
	headers := []string{
		"Upgrade",
		"Connection",
		"Sec-Websocket-Key",
		"Sec-Websocket-Version",
		"Sec-Websocket-Extensions",
	}
	for _, value := range headers {
		if ok := i.request.Header.Get(value); ok != "" {
			i.request.Header.Del(value)
		}
	}
}
