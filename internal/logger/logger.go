package logger

import (
	"log"
	"os"
)

type Level int

const (
	Silent Level = iota
	Info
	Debug
)

var currentLevel Level = Info

func SetLevel(level Level) {
	currentLevel = level
}

func Infof(format string, v ...any) {
	if currentLevel >= Info {
		log.Printf("INFO: "+format, v...)
	}
}

func Debugf(format string, v ...any) {
	if currentLevel >= Debug {
		log.Printf("DBG: "+format, v...)
	}
}

func Errorf(format string, v ...any) {
	log.Printf("ERR: "+format, v...)
}

func Fatalf(format string, v ...any) {
	log.Fatalf("FATAL: "+format, v...)
}

func Warnf(format string, v ...any) {
	if currentLevel >= Info {
		log.Printf("WARN: "+format, v...)
	}
}

func Init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime)
}
