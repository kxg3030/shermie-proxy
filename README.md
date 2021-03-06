#### 一、项目
> 使用go开发的代理工具，目前支持http、https、ws、wss、tcp、socket5等协议

- 支持多种协议的数据接收发送，只需要监听一个端口
- 支持数据拦截和自定义修改
- 自动识别入站协议，按照消息头识别不同的协议进行处理
- 支持添加代理

#### 二、使用

- 安装
```go
go get github.com/kxg3030/shermie-proxy
```

- 监听服务
```go
func init() {
 	// 初始化日志
 	Log.NewLogger().Init()
 	// 初始化根证书
 	err := Core.NewCertificate().Init()
 	if err != nil {
 		Log.Log.Println("初始化根证书失败：" + err.Error())
 		return
 	}
}
func main() {
         // 启动服务
   	s := Core.NewProxyServer(*port)
   	// 注册http事件函数
   	s.OnRequestEvent = func(request *http.Request) {
   		//fmt.Println("http请求地址：" + request.URL.Host)
   	}
   	// 注册http事件函数
   	s.OnResponseEvent = func(response *http.Response) {
   		contentType := response.Header.Get("Content-Type")
   		var reader io.Reader
   		if strings.Contains(contentType, "json") {
   			reader = bufio.NewReader(response.Body)
   			if header := response.Header.Get("Content-Encoding"); header == "gzip" {
   				reader, _ = gzip.NewReader(response.Body)
   			}
   			body, _ := io.ReadAll(reader)
   			fmt.Println("http返回数据：" + string(body))
   		}
   	}
   	// 注册socket5服务器向客户端推送消息事件函数
   	s.OnServerResponseEvent = func(message []byte) {
   		fmt.Println("socket5服务器发送数据", message)
   	}
   	// 注册socket5客户端向服务器推送消息事件函数
   	s.OnClientSendEvent = func(message []byte) {
   		fmt.Println("socket5客户端发送数据", message)
   	}
   	// 注册ws服务器向客户端推送消息事件函数
   	s.OnServerPacketEvent = func(msgType int, message []byte, clientConn *Websocket.Conn, resolve Core.ResolveWs) error {
   		fmt.Println("服务器向浏览器响应数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
   		return clientConn.WriteMessage(msgType, message)
   	}
   	// 注册ws客户端向服务器推送消息事件函数
   	s.OnClientPacketEvent = func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve Core.ResolveWs) error {
   		fmt.Println("浏览器向服务器发送数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
   		return resolve(msgType, message, tartgetConn)
   	}
   	_ = s.Start()
}
```

#### 三、问题
如果遇到报错`sslConn.ReadLastTimeBytes undefined (type *tls.Conn has no field or method ReadLastTimeBytes)`,将下面的方法添加到你的gopath路径下src/crypto/tls/conn.go文件中,这个方法是获取服务器与客户端tls握手后，客户端发送的最后一帧原始数据。
```go
func (c *Conn) ReadLastTimeBytes() []byte { 
    return c.rawInput.Bytes() 
}
```
