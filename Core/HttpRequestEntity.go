package Core

import (
	"io"
	"net/http"
	"time"
)

type HttpRequestEntity struct {
	startTime time.Time
	request   *http.Request
	body      io.ReadCloser
}
