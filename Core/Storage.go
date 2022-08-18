package Core

import (
	"crypto/tls"
	"net"
	"strings"
	"sync"
)

var Cache = NewStorage()

type Action struct {
	wg   *sync.WaitGroup
	fn   func() (interface{}, error)
	cert interface{}
	err  error
}

type Storage struct {
	lock   *sync.Mutex
	buffer map[string]*Action
}

func NewStorage() *Storage {
	return &Storage{
		lock:   &sync.Mutex{},
		buffer: map[string]*Action{},
	}
}

func (i *Storage) GetCertificate(hostname string, port string) (interface{}, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if strings.Index(hostname, ":") == -1 {
		hostname += ":" + port
	}
	host, _, err := net.SplitHostPort(hostname)
	if err != nil {
		return nil, err
	}
	if action, exist := i.buffer[host]; exist {
		return action.cert, nil
	}
	// 如果不存在，单个域名只需要生成一次
	i.buffer[host] = &Action{
		wg: &sync.WaitGroup{},
		fn: GetAction(host),
	}
	i.buffer[host].cert, i.buffer[host].err = i.buffer[host].fn()
	defer func() {
		//i.lock.Lock()
		//delete(i.buffer, host)
		//i.lock.Unlock()
	}()
	return i.buffer[host].cert, i.buffer[host].err
}

func GetAction(hostname string) func() (interface{}, error) {
	return func() (interface{}, error) {
		// 为每个host:port生成单独的证书
		cert, privateKey, err := Cert.GeneratePem(hostname)
		if err != nil {
			return nil, err
		}
		// 生成证书
		certificate, err := tls.X509KeyPair(cert, privateKey)
		if err != nil {
			return nil, err
		}
		return certificate, nil
	}
}
