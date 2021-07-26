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
	logLevel                        LogLevel
	debug, info, warn, err, success *pterm.PrefixPrinter
	spinner                         *pterm.SpinnerPrinter
	table                           *pterm.TablePrinter
	section                         *pterm.SectionPrinter
	progress                        *ProgressbarPrinter
}

func NewLogger() Logger {
	debug := pterm.Debug.WithDebugger(false).WithPrefix(pterm.Prefix{
		Style: &pterm.ThemeDefault.DebugPrefixStyle,
		Text:  "DEBG",
	})
	warn := pterm.Warning.WithPrefix(pterm.Prefix{
		Style: &pterm.ThemeDefault.WarningPrefixStyle,
		Text:  "WARN",
	})
	err := pterm.Error.WithShowLineNumber(false).WithPrefix(pterm.Prefix{
		Style: &pterm.ThemeDefault.ErrorPrefixStyle,
		Text:  "ERR ",
	})
	success := pterm.Success.WithPrefix(pterm.Prefix{
		Style: &pterm.ThemeDefault.SuccessPrefixStyle,
		Text:  " OK ",
	})

	spinner := pterm.DefaultSpinner
	spinner.SuccessPrinter = success
	spinner.WarningPrinter = warn
	spinner.FailPrinter = err

	table := pterm.DefaultTable
	section := pterm.DefaultSection
	progress := ProgressbarPrinter{ProgressbarPrinter: pterm.DefaultProgressbar.WithRemoveWhenDone(true)}

	l := &Log{
		logLevel: LogLevelDebug,
		debug:    debug,
		info:     &pterm.Info,
		warn:     warn,
		err:      err,
		success:  success,

		spinner:  &spinner,
		table:    &table,
		section:  &section,
		progress: &progress,
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
	l.info.Printf(format, a...)
}

func (l *Log) Infoln(a ...interface{}) {
	l.info.Println(a...)
}

func (l *Log) Successf(format string, a ...interface{}) {
	l.success.Printf(format, a...)
}

func (l *Log) Successln(a ...interface{}) {
	l.success.Println(a...)
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

func (l *Log) Spinner() pterm.SpinnerPrinter {
	return *l.spinner
}

func (l *Log) ProgressBar() ProgressbarPrinter {
	return *l.progress
}

func (l *Log) Table() pterm.TablePrinter {
	return *l.table
}

func (l *Log) Section() *pterm.SectionPrinter {
	return l.section
}
