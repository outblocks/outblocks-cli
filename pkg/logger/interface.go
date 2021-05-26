package logger

import "github.com/pterm/pterm"

type Logger interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Printo(a ...interface{})

	Debugf(format string, a ...interface{})
	Debugln(a ...interface{})
	Infof(format string, a ...interface{})
	Infoln(a ...interface{})
	Successf(format string, a ...interface{})
	Successln(a ...interface{})
	Warnf(format string, a ...interface{})
	Warnln(a ...interface{})
	Errorf(format string, a ...interface{})
	Errorln(a ...interface{})

	Level() LogLevel
	SetLevel(logLevel string) error

	Spinner() pterm.SpinnerPrinter
	ProgressBar() pterm.ProgressbarPrinter
	Table() pterm.TablePrinter
}
