package Core

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"github/shermie-proxy/Log"
	"io"
	"net"
	"net/http"
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
	// 如果是connect方法则是ssl请求
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
	httpEntity := &HttpRequestEntity{
		startTime: time.Now(),
		request:   i.request,
	}
	// TODO 处理ws

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

// 处理客户端的[server-hello]请求
func (i *ProxyHttp) handleSslRequest() {
	// 预先测试一下目标服务器能否连接
	target, err := net.Dial("tcp", i.request.Host)
	if err != nil {
		Log.Log.Println("连接目的地址失败：" + err.Error())
		return
	}
	defer target.Close()
	// 向源连接返回连接成功(这里如果使用i.writer.Write后面必须马上flush,否则数据还是存在于缓冲区里面)
	_, err = i.conn.Write([]byte(ConnectSuccess))
	if err != nil {
		Log.Log.Println("返回连接状态失败：" + err.Error())
		return
	}
	// 建立ssl连接并返回给源
	i.SslReceiveSend()
}

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
	// 创建ssl连接
	sslConn := tls.Server(i.conn, &tls.Config{Certificates: []tls.Certificate{cert}})
	err = sslConn.Handshake()
	if err != nil {
		if err == io.EOF || strings.Index(err.Error(), "An existing connection was forcibly closed by the remote host.") != -1 {
			Log.Log.Println("客户端连接超时" + err.Error())
			return
		}
		Log.Log.Println("其他错误是个什么错误：" + err.Error())
		// 获取最后返回数据
		//lastMsg := string(sslConn.ReadLastTimeBytes())
		//if lastMsg == "" {
		//	target, err := net.Dial("tcp", i.request.Host)
		//	if err != nil {
		//		Log.Log.Println("connect remote  http server error：" + err.Error())
		//		return
		//	}
		//	defer target.Close()
		//	targetWriter := bufio.NewWriter(target)
		//	_, _ = targetWriter.Write([]byte(lastMsg))
		//	// 复制数据：源-->目
		//	_, err = io.Copy(targetWriter, i.reader)
		//	_ = targetWriter.Flush()
		//	if err != nil {
		//		Log.Log.Println("source copy to destination error：" + err.Error())
		//		return
		//	}
		//	// 复制数据：源<--目
		//	_, err = io.Copy(i.writer, target)
		//	_ = i.writer.Flush()
		//	if err != nil {
		//		Log.Log.Println("destination copy to source  error：" + err.Error())
		//		return
		//	}
		//}
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
	body, _ := i.NopBuffReader(i.request.Body)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	i.server.OnRequestEvent(i.request)
	i.request.Body = io.NopCloser(bytes.NewReader(body))
	httpEntity := &HttpRequestEntity{
		startTime: time.Now(),
		request:   i.request,
		body:      i.request.Body,
	}
	httpEntity.request.URL.Host = i.request.Host
	httpEntity.request.RemoteAddr = i.request.RemoteAddr
	httpEntity.request.URL.Scheme = "https"
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
