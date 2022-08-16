package Utils

import (
	"bytes"
	"crypto/tls"
	"os"
	"reflect"
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
