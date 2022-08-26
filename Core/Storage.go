package Core

import (
	"crypto/tls"
	"net"
	"strings"
	"sync"
)

var Cache = NewStorage()

type Action struct {
	wg     *sync.WaitGroup
	fn     func() (interface{}, error)
	cert   interface{}
	forget bool
	err    error
}

type Storage struct {
	lock    *sync.Mutex
	mapping map[string]*Action
}

func NewStorage() *Storage {
	return &Storage{
		lock:    &sync.Mutex{},
		mapping: map[string]*Action{},
	}
}

func (i *Storage) do(action *Action, host string, callback func() (interface{}, error)) {
	defer func() {
		action.wg.Done()
	}()
	action.cert, action.err = callback()
}

func (i *Storage) GetCertificate(hostname string, port string) (interface{}, error) {
	i.lock.Lock()
	if strings.Index(hostname, ":") == -1 {
		hostname += ":" + port
	}
	host, _, err := net.SplitHostPort(hostname)
	if err != nil {
		return nil, err
	}
	// 对相同域名的并发,同一时刻只生成一个证书
	if action, exist := i.mapping[host]; exist {
		i.lock.Unlock()
		action.wg.Wait()
		return action.cert, nil
	}
	// 对不同的域名的并发,同一时刻只生成一个域名处理对象
	i.mapping[host] = &Action{
		wg: &sync.WaitGroup{},
		fn: GetAction(host),
	}
	i.mapping[host].wg.Add(1)
	i.lock.Unlock()
	i.do(i.mapping[host], host, i.mapping[host].fn)
	return i.mapping[host].cert, i.mapping[host].err
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
