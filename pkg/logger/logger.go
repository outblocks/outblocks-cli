package logger

import (
	"fmt"
	"runtime"
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
	debug, info, warn, err *pterm.PrefixPrinter
}

func NewLogger() Logger {
	l := &Log{
		logLevel: LogLevelDebug,
		debug:    pterm.Debug.WithDebugger(false),
		info:     &pterm.Info,
		warn:     &pterm.Warning,
		err:      pterm.Error.WithShowLineNumber(false),
	}

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

	if l.logLevel == LogLevelDebug {
		_, fileName, line, _ := runtime.Caller(1)

		pterm.Println(pterm.FgGray.Sprint("└ " + fmt.Sprintf("(%s:%d)", fileName, line)))
	}
}

func (l *Log) Errorln(a ...interface{}) {
	l.Errorf(fmt.Sprintln(a...))
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

	l.logLevel = level

	return nil
}

func (l *Log) Level() LogLevel {
	return l.logLevel
}
