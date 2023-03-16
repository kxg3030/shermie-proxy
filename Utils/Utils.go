package Utils

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"unsafe"
)

func FileExist(file string) bool {
	_, err := os.Stat(file)
	if err != nil {
		return false
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func GetLastTimeFrame(conn *tls.Conn, property string) []byte {
	rawInputPtr := reflect.ValueOf(conn).Elem().FieldByName(property)
	if rawInputPtr.Kind() != reflect.Struct {
		return []byte{}
	}
	val, _ := reflect.NewAt(rawInputPtr.Type(), unsafe.Pointer(rawInputPtr.UnsafeAddr())).Elem().Interface().(bytes.Buffer)
	return val.Bytes()
}

func InstallCert(certName string) error {
	if runtime.GOOS != "windows" {
		return errors.New("非windows系统请手动安装证书")
	}
	current, _ := os.Getwd()
	certPath := filepath.Join(current, certName)
	fmt.Println(certPath)
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("读取证书文件错误：%w", err)
	}
	block, _ := pem.Decode(cert)
	certBytes := block.Bytes
	certContext, err := windows.CertCreateCertificateContext(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		&certBytes[0],
		uint32(len(certBytes)))
	if err != nil {
		return fmt.Errorf("创建证书上下文错误：%w", err)
	}
	defer func() {
		_ = windows.CertFreeCertificateContext(certContext)
	}()
	utf16Ptr, err := windows.UTF16PtrFromString("Root")
	storeHandle, err := windows.CertOpenSystemStore(0, utf16Ptr)
	if err != nil {
		return fmt.Errorf("打开系统证书存储区失败：%w", err)
	}
	defer func() {
		_ = windows.CertCloseStore(storeHandle, windows.CERT_CLOSE_STORE_FORCE_FLAG)
	}()

	err = windows.CertAddCertificateContextToStore(storeHandle, certContext, windows.CERT_STORE_ADD_REPLACE_EXISTING_INHERIT_PROPERTIES, nil)
	if err != nil {
		return fmt.Errorf("安装结果：%w", err)
	}
	return nil
}
