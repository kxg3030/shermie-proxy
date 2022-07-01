package Log

import (
	"log"
	"os"
)

var Log *log.Logger

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func (i *Logger) Init() {
	Log = log.New(os.Stdout, "", log.Ldate | log.Ltime)
}
