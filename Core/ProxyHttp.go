package Core

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"github/shermie-proxy/Core/Websocket"
	"github/shermie-proxy/Log"
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
const SslFileHost = "zt.io"

type ProxyHttp struct {
	server   *ProxyServer
	writer   *bufio.Writer
	reader   *bufio.Reader
	request  *http.Request
	response *http.Response
	upgrade  Websocket.Upgrader
	ssl      bool
	conn     net.Conn
	port     string
}

func NewProxyHttp() *ProxyHttp {
	p := &ProxyHttp{}
	return p
}

func (i *ProxyHttp) handle() {
	request, err := http.ReadRequest(i.reader)
	if err != nil {
		return
	}
	// 给个默认端口
	i.port = "-1"
	if hostname := strings.Split(request.Host, ":"); len(hostname) > 1 {
		i.port = hostname[len(hostname)-1]
	}
	i.request = request
	// 如果是connect方法则是http协议的ssl请求或者ws请求
	if i.request.Method == http.MethodConnect {
		i.handleSslRequest()
		return
	}
	// 否则是普通请求
	i.handleRequest()
}

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
	body, _ := i.NopBuffReader(i.request.Body)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnRequestEvent(i.request)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	httpEntity := &HttpRequestEntity{startTime: time.Now(), request: i.request,}
	// TODO 处理ws
	if ok := i.handleWsRequest(); ok {
		return
	}
	// 处理正常请求,获取响应
	i.response, err = i.Transport(httpEntity)
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
	body, _ = i.NopBuffReader(i.response.Body)
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

func (i *ProxyHttp) NopBuffReader(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(reader)
	return body, err
}

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
		// 代理层不能转发这个首部
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

func (i *ProxyHttp) Transport(httpEntity *HttpRequestEntity) (*http.Response, error) {
	// 去除一些头部
	i.RemoveHeader(httpEntity.request.Header)
	response, err := (&http.Transport{
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 60 * time.Second,
	}).RoundTrip(httpEntity.request)
	if err != nil {
		return nil, err
	}
	// 去除一些头部
	i.RemoveHeader(response.Header)
	return response, err
}

func (i *ProxyHttp) handleSslRequest() {
	// 预先测试一下目标服务器能否连接
	target, err := net.Dial("tcp", i.request.Host)
	if err != nil {
		Log.Log.Println("连接目的地址失败：" + err.Error())
		return
	}
	defer target.Close()
	// 向源连接返回连接成功
	_, err = i.conn.Write([]byte(ConnectSuccess))
	if err != nil {
		Log.Log.Println("返回连接状态失败：" + err.Error())
		return
	}
	// 建立ssl连接并返回给源
	i.SslReceiveSend(target)
}

func (i *ProxyHttp) SetRequest(request *http.Request) *http.Request {
	request.Header.Set("Connection", "false")
	request.URL.Host = request.Host
	request.URL.Scheme = "https"
	return request
}

func (i *ProxyHttp) SslReceiveSend(target net.Conn) {
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
	// 创建ssl连接
	sslConn := tls.Server(i.conn, &tls.Config{Certificates: []tls.Certificate{cert}})
	err = sslConn.Handshake()
	// 如果不是http的ssl握手请求,则说明是ws请求,这里专门处理这种情况
	if err != nil {
		if err == io.EOF || strings.Index(err.Error(), "An existing connection was forcibly closed by the remote host.") != -1 {
			Log.Log.Println("客户端连接超时" + err.Error())
			return
		}
		// 获取浏览器发送给服务器的头部和数据,构建一个完整的请求对象
		rawInput := string(sslConn.ReadLastTimeBytes())
		rawInputList := strings.Split(rawInput, "\r\n")
		_ = i.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		// 读取请求方法
		wsMethodList := strings.Split(rawInputList[0], " ")
		// 构建请求
		wsRequest := &http.Request{
			Method: wsMethodList[0],
			Header: map[string][]string{},
		}
		for _, value := range rawInputList {
			// 填充header
			headerKeValList := strings.Split(value, ":")
			if len(headerKeValList) > 1 {
				wsRequest.Header.Set(headerKeValList[0], headerKeValList[1])
			}
			// 填充host
			if wsRequest.RequestURI == "" && headerKeValList[0] == "HOST" {
				wsRequest.Host = headerKeValList[1]
				// 填充path
				wsRequest.RequestURI = fmt.Sprintf("%s:%s", headerKeValList[0], wsMethodList[1])
				wsRequest.URL, _ = url.Parse(fmt.Sprintf("%s:%s", headerKeValList[0], wsMethodList[1]))
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
		// 将构建的请求对象
		if wsMethodList[0] == http.MethodConnect {
			i.handleSslRequest()
			return
		}
		i.handleRequest()
		return
	}
	_ = sslConn.SetDeadline(time.Now().Add(time.Second * 60))
	defer func() {
		_ = sslConn.Close()
	}()
	i.request, err = http.ReadRequest(bufio.NewReader(sslConn))
	if err != nil {
		Log.Log.Println("读取ssl连接请求数据失败：" + err.Error())
		return
	}
	i.request = i.SetRequest(i.request)
	body, _ := i.NopBuffReader(i.request.Body)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnRequestEvent(i.request)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	httpEntity := &HttpRequestEntity{startTime: time.Now(), request: i.request, body: i.request.Body,}
	i.response, err = i.Transport(httpEntity)
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
		Log.Log.Println("远程服务器响应失败：" + err.Error())
		return
	}
	body, _ = i.NopBuffReader(i.response.Body)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnResponseEvent(i.response)
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	// 如果写入的数据比返回的头部指定长度还长,就会报错,这里手动计算返回的数据长度
	i.response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	i.response.Body = io.NopCloser(bytes.NewReader(body))
	err = i.response.Write(sslConn)
	if err != nil {
		Log.Log.Println("代理返回响应数据失败：" + err.Error())
	}
}

func (i *ProxyHttp) handleWsRequest() bool {

	if i.request.Header.Get("Upgrade") == "" {
		return false
	}
	const WssConnectionok = 0x3
	const WssUserSend = 0x4
	const WssServerSend = 0x5
	const WssDisconnect = 0x6
	const WsConnectionok = 0x7
	const WsUserSend = 0x8
	const WsServerSend = 0x9
	const WsDisconnect = 0xA
	i.upgrade = Websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{i.request.Header.Get("Sec-WebSocket-Protocol")},
	}
	recorder := httptest.NewRecorder()
	readerWriter := bufio.NewReadWriter(i.reader, i.writer)
	// 升级成ws连接
	wsConn, err := i.upgrade.Upgrade(recorder, i.request, nil, i.conn, readerWriter)
	if err != nil {
		return true
	}
	defer func() {
		_ = wsConn.Close()
	}()
	hostname := fmt.Sprintf("%s://%s%s?%s", func() string {
		if i.ssl {
			return "wss"
		}
		return "ws"
	}(), i.request.Host, i.request.URL.Path, i.request.URL.RawQuery)
	fmt.Println(hostname)
	return false
}
