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
 	s := Core.NewProxyServer("9090")
 	// 注册http事件函数
 	s.OnRequestEvent = func(request *http.Request) {
 
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
 			fmt.Println(string(body))
 		}
 	}
 	// 注册ws事件函数
 	s.OnServerPacketEvent = func(msgType int, message []byte, clientConn *Websocket.Conn, resolve Core.ResolveWs) error {
 		fmt.Println("服务器向浏览器响应数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
        // 手动发送ws消息(适用于需要对消息进行剪裁处理的情况)
 		return clientConn.WriteMessage(msgType,message)
 	}
 	s.OnClientPacketEvent = func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve Core.ResolveWs) error {
 		fmt.Println("浏览器向服务器发送数据：" + string(message) + "消息号：" + strconv.Itoa(msgType))
        // 让框架代为处理数据(默认的行为)
 		return resolve(msgType,message,tartgetConn)
 	}
 	_ = s.Start()
}
```
