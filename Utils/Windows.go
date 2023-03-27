//go:build (windows)
// +build windows

package Utils

import (
	"encoding/pem"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const UnProxy = "localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*"

func InstallCert(certName string) error {
	current, _ := os.Getwd()
	certPath := filepath.Join(current, certName)
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
		return fmt.Errorf("安装系统证书失败：%w", err)
	}
	return nil
}

func SetWindowsProxy(proxy string) error {
	const (
		InternetPerConnFlags              = 1
		InternetPerConnProxyServer        = 2
		InternetPerConnProxyBypass        = 3
		InternetOptionRefresh             = 37
		InternetOptionSettingsChanged     = 39
		InternetOptionPerConnectionOption = 75
	)

	type InternetPerConnOption struct {
		dwOption uint32
		dwValue  uint64
	}

	type InternetPerConnOptionList struct {
		dwSize        uint32
		pszConnection *uint16
		dwOptionCount uint32
		dwOptionError uint32
		pOptions      uintptr
	}
	winInet, err := windows.LoadLibrary("Wininet.dll")
	if err != nil {
		return fmt.Errorf("加载动态库失败: %w", err)
	}
	InternetSetOptionW, err := windows.GetProcAddress(winInet, "InternetSetOptionW")
	if err != nil {
		return fmt.Errorf("获取函数地址错误: %w", err)
	}
	options := [3]InternetPerConnOption{}
	options[0].dwOption = InternetPerConnFlags
	if proxy == "" {
		options[0].dwValue = 1
	} else {
		options[0].dwValue = 2
	}
	options[1].dwOption = InternetPerConnProxyServer
	options[1].dwValue = uint64(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(proxy))))
	options[2].dwOption = InternetPerConnProxyBypass

	options[2].dwValue = uint64(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(UnProxy))))

	list := InternetPerConnOptionList{}
	list.dwSize = uint32(unsafe.Sizeof(list))
	list.pszConnection = nil
	list.dwOptionCount = 3
	list.dwOptionError = 0
	list.pOptions = uintptr(unsafe.Pointer(&options))
	callInternetOptionW := func(dwOption uintptr, lpBuffer uintptr, dwBufferLength uintptr) error {
		r1, _, err := syscall.Syscall6(InternetSetOptionW, 4, 0, dwOption, lpBuffer, dwBufferLength, 0, 0)
		if r1 != 1 {
			return fmt.Errorf("调用库函数失败: %w", err)
		}
		return nil
	}

	err = callInternetOptionW(InternetOptionPerConnectionOption, uintptr(unsafe.Pointer(&list)), unsafe.Sizeof(list))
	if err != nil {
		return fmt.Errorf("设置系统代理失败: %w", err)
	}
	err = callInternetOptionW(InternetOptionSettingsChanged, 0, 0)
	if err != nil {
		return fmt.Errorf("修改系统代理失败: %s", err)
	}
	err = callInternetOptionW(InternetOptionRefresh, 0, 0)
	if err != nil {
		return fmt.Errorf("刷新系统代理失败: %s", err)
	}
	return nil
}
