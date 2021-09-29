package logger

import "github.com/pterm/pterm"

type Logger interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Printo(a ...interface{})
	StderrPrintf(format string, a ...interface{})
	StderrPrintln(a ...interface{})
	StderrPrinto(a ...interface{})

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
	SetLevel(logLevel LogLevel)
	SetLevelString(logLevel string) error

	Spinner() pterm.SpinnerPrinter
	Table() pterm.TablePrinter
	Section() *pterm.SectionPrinter
	ProgressBar() ProgressbarPrinter
}
