package logger

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type Logger struct {
	logLevel               LogLevel
	debug, info, warn, err pterm.PrefixPrinter
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	if l.logLevel > LogLevelDebug {
		return
	}

	l.debug.Printf(format, a...)
}

func (l *Logger) Debugln(a ...interface{}) {
	if l.logLevel > LogLevelDebug {
		return
	}

	l.debug.Println(a...)
}

func (l *Logger) Infof(format string, a ...interface{}) {
	if l.logLevel > LogLevelInfo {
		return
	}

	l.info.Printf(format, a...)
}

func (l *Logger) Infoln(a ...interface{}) {
	if l.logLevel > LogLevelInfo {
		return
	}

	l.info.Println(a...)
}

func (l *Logger) Warnf(format string, a ...interface{}) {
	if l.logLevel > LogLevelWarn {
		return
	}

	l.warn.Printf(format, a...)
}

func (l *Logger) Warnln(a ...interface{}) {
	if l.logLevel > LogLevelWarn {
		return
	}

	l.warn.Println(a...)
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	l.err.Printf(format, a...)
}

func (l *Logger) Errorln(a ...interface{}) {
	l.err.Println(a...)
}

func (l *Logger) SetLevel(logLevel string) error {
	var level LogLevel

	switch strings.ToLower(logLevel) {
	case "debug":
		level = LogLevelDebug
	case "info", "":
		level = LogLevelInfo
	case "warn", "warning":
		level = LogLevelWarn
	case "error":
		level = LogLevelError
	default:
		return fmt.Errorf("unknown log level specified")
	}

	l.err.ShowLineNumber = level == LogLevelDebug

	return nil
}

func (l *Logger) Level() LogLevel {
	return l.logLevel
}

func NewLogger() *Logger {
	l := &Logger{
		logLevel: LogLevelDebug,
		debug:    pterm.Debug,
		info:     pterm.Info,
		warn:     pterm.Warning,
		err:      pterm.Error,
	}

	l.debug.Debugger = false

	return l
}
