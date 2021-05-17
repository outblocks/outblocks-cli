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

type Log struct {
	logLevel               LogLevel
	debug, info, warn, err pterm.PrefixPrinter
}

func NewLogger() Logger {
	l := &Log{
		logLevel: LogLevelDebug,
		debug:    pterm.Debug,
		info:     pterm.Info,
		warn:     pterm.Warning,
		err:      pterm.Error,
	}

	l.debug.Debugger = false

	return l
}

func (l *Log) Printf(format string, a ...interface{}) {
	pterm.Printf(format, a...)
}

func (l *Log) Println(a ...interface{}) {
	pterm.Println(a...)
}

func (l *Log) Printo(a ...interface{}) {
	pterm.Printo(a...)
}

func (l *Log) Debugf(format string, a ...interface{}) {
	if l.logLevel > LogLevelDebug {
		return
	}

	l.debug.Printf(format, a...)
}

func (l *Log) Debugln(a ...interface{}) {
	if l.logLevel > LogLevelDebug {
		return
	}

	l.debug.Println(a...)
}

func (l *Log) Infof(format string, a ...interface{}) {
	if l.logLevel > LogLevelInfo {
		return
	}

	l.info.Printf(format, a...)
}

func (l *Log) Infoln(a ...interface{}) {
	if l.logLevel > LogLevelInfo {
		return
	}

	l.info.Println(a...)
}

func (l *Log) Warnf(format string, a ...interface{}) {
	if l.logLevel > LogLevelWarn {
		return
	}

	l.warn.Printf(format, a...)
}

func (l *Log) Warnln(a ...interface{}) {
	if l.logLevel > LogLevelWarn {
		return
	}

	l.warn.Println(a...)
}

func (l *Log) Errorf(format string, a ...interface{}) {
	l.err.Printf(format, a...)
}

func (l *Log) Errorln(a ...interface{}) {
	l.err.Println(a...)
}

func (l *Log) SetLevel(logLevel string) error {
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

func (l *Log) Level() LogLevel {
	return l.logLevel
}
