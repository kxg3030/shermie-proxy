//go:build linux
// +build linux

package Utils

import "errors"

func InstallCert(certName string) error {

	return errors.New("不支持Linux系统")
}

func SetSystemProxy(proxy string) error {

	return errors.New("不支持Linux系统")
}
