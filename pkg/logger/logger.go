package logger

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/gookit/color"
	"github.com/outblocks/outblocks-cli/internal/util"
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
	debug, info, warn, err, success *PrefixPrinter
	spinner                         *SpinnerPrinter
	table                           *TablePrinter
	section                         *SectionPrinter
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
	err := pterm.Error.WithPrefix(pterm.Prefix{
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
	spinner.RemoveWhenDone = true

	table := pterm.DefaultTable
	section := pterm.DefaultSection
	progress := ProgressbarPrinter{ProgressbarPrinter: pterm.DefaultProgressbar.WithRemoveWhenDone(true)}

	l := &Log{
		logLevel: LogLevelDebug,
		debug:    &PrefixPrinter{PrefixPrinter: debug},
		info:     &PrefixPrinter{PrefixPrinter: &pterm.Info},
		warn:     &PrefixPrinter{PrefixPrinter: warn},
		err:      &PrefixPrinter{PrefixPrinter: err},
		success:  &PrefixPrinter{PrefixPrinter: success},

		spinner:  &SpinnerPrinter{SpinnerPrinter: spinner},
		table:    &TablePrinter{TablePrinter: &table},
		section:  &SectionPrinter{SectionPrinter: section},
		progress: &progress,
	}

	return l
}

func printer(a ...interface{}) {
	if util.IsTermDumb() {
		if !pterm.Output {
			return
		}

		color.Print(color.Sprint(pterm.Sprint(a...)))

		return
	}

	pterm.Print(a...)
}
func (l *Log) Print(a ...interface{}) {
	printer(a...)
}

func (l *Log) Printf(format string, a ...interface{}) {
	l.Print(pterm.Sprintf(format, a...))
}

func (l *Log) Println(a ...interface{}) {
	l.Print(pterm.Sprintln(a...))
}

func (l *Log) Printo(a ...interface{}) {
	if util.IsTermDumb() {
		l.Println(a...)

		return
	}

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
	l.err.Println(a...)

	if l.logLevel == LogLevelDebug {
		_, fileName, line, _ := runtime.Caller(1)
		pterm.Println(pterm.FgGray.Sprint("└ " + fmt.Sprintf("(%s:%d)", fileName, line)))
	}
}

func (l *Log) SetLevelString(logLevel string) error {
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

func (l *Log) SetLevel(logLevel LogLevel) {
	l.logLevel = logLevel
}

func (l *Log) Level() LogLevel {
	return l.logLevel
}

func (l *Log) Spinner() Spinner {
	return l.spinner
}

func (l *Log) ProgressBar() Progressbar {
	return l.progress
}

func (l *Log) Table() Table {
	return l.table
}

func (l *Log) Section() Section {
	return l.section
}
