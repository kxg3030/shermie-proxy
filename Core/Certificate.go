package Core

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Constant"
	"github.com/kxg3030/shermie-proxy/Utils"
	"math/big"
	"net"
	"os"
	"time"
)

var Cert *Certificate

type Certificate struct {
	RootKey    *rsa.PrivateKey
	RootCa     *x509.Certificate
	RootCaStr  []byte
	RootKeyStr []byte
}

func NewCertificate() *Certificate {
	return &Certificate{
		RootKey: nil,
		RootCa:  nil,
	}
}

// 初始化根证书
func (i *Certificate) Init() error {
	var err error
	var certBlock, keyBlock *pem.Block
	// 如果根证书不存在,则生成
	if !Utils.FileExist("./cert.crt") {
		// 生成根pem文件
		certBlock, keyBlock, err = i.GenerateRootPemFile("Shermie")
		if err != nil {
			return fmt.Errorf("生成根证书文件失败：%w", err)
		}
	} else {
		// 根证书存在,则使用
		certBlock, _ = pem.Decode([]byte(Constant.RootCert))
		keyBlock, _ = pem.Decode([]byte(Constant.RootPriKey))
	}
	i.RootKeyStr = keyBlock.Bytes
	i.RootCaStr = certBlock.Bytes
	i.RootCa, err = x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("初始化根根证书失败：%w", err)
	}
	i.RootKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("初始化根根证书私钥失败：%w", err)
	}
	Cert = i
	return nil
}

// 用根证书生成新的子证书
func (i *Certificate) GeneratePem(host string) ([]byte, []byte, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)   //把 1 左移 128 位，返回给 big.Int
	serialNumber, _ := rand.Int(rand.Reader, max) //返回在 [0, max) 区间均匀随机分布的一个随机值
	template := x509.Certificate{
		SerialNumber: serialNumber, // SerialNumber 是 CA 颁布的唯一序列号，在此使用一个大随机数来代表它
		Subject: pkix.Name{ // Name代表一个X.509识别名。只包含识别名的公共属性，额外的属性被忽略。
			Country:            []string{"CN"},         // 证书所属的国家
			Organization:       []string{"company"},    // 证书存放的公司名称
			OrganizationalUnit: []string{"department"}, // 证书所属的部门名称
			Province:           []string{"BeiJing"},    // 证书签发机构所在省
			CommonName:         host,
			Locality:           []string{"BeiJing"}, // 证书签发机构所在市
		},
		NotBefore:             time.Now().AddDate(-1, 0, 0),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		EmailAddresses:        []string{"forward.nice.cp@gmail.com"},
		BasicConstraintsValid: true,
		IsCA:                  true,
		Issuer: pkix.Name{
			CommonName: host,
		},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	priKey, err := i.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, i.RootCa, &priKey.PublicKey, i.RootKey)
	if err != nil {
		return nil, nil, err
	}
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}
	priKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priKey),
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(priKeyBlock), err
}

func (i *Certificate) GenerateRootPemFile(host string) (*pem.Block, *pem.Block, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, max)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"CN"},         // 证书所属的国家
			Organization:       []string{"company"},    // 证书存放的公司名称
			OrganizationalUnit: []string{"department"}, // 证书所属的部门名称
			Province:           []string{"BeiJing"},    // 证书签发机构所在省
			CommonName:         host,
			Locality:           []string{"BeiJing"}, // 证书签发机构所在市
		},
		NotBefore:             time.Now().AddDate(-1, 0, 0),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		EmailAddresses:        []string{"forward.nice.cp@gmail.com"},
		BasicConstraintsValid: true,
		IsCA:                  true,
		Issuer: pkix.Name{
			CommonName: host,
		},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	priKey, err := i.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &priKey.PublicKey, priKey)
	if err != nil {
		return nil, nil, err
	}

	// 将私钥写入.key文件
	keyFd, _ := os.OpenFile("./cert.key", os.O_WRONLY|os.O_CREATE, os.ModePerm.Perm())
	defer func() {
		err = keyFd.Close()
	}()
	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priKey),
	}
	_ = pem.Encode(keyFd, keyBlock)
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}
	// 将证书写入.crt文件
	certFd, _ := os.OpenFile("./cert.crt", os.O_WRONLY|os.O_CREATE, os.ModePerm.Perm())
	defer func() {
		_ = certFd.Close()
	}()
	_ = pem.Encode(certFd, certBlock)
	return certBlock, keyBlock, err
}

// 生成一对具有指定字位数的RSA密钥
func (i *Certificate) GenerateKeyPair() (*rsa.PrivateKey, error) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("密钥对生成失败")
	}
	return priKey, nil
}
