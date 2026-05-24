package logger

import (
	"log"
	"os"
)

type Level int

const (
	SilentL Level = iota
	NormalL
	DebugL
)

var currentLevel Level = NormalL

func SetLevel(level Level) {
	currentLevel = level
}

func Info(format string, v ...any) {
	if currentLevel >= NormalL {
		log.Printf("INFO: "+format, v...)
	}
}

func Debug(format string, v ...any) {
	if currentLevel >= DebugL {
		log.Printf("DBG: "+format, v...)
	}
}

func Error(format string, v ...any) {
	log.Printf("ERR: "+format, v...)
}

func Fatal(format string, v ...any) {
	log.Fatalf("FATAL: "+format, v...)
}

func Warn(format string, v ...any) {
	if currentLevel >= NormalL {
		log.Printf("WARN: "+format, v...)
	}
}

func Init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime)
}
