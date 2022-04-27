package logger

import (
	"time"

	"github.com/pterm/pterm"
)

type Logger interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Printo(a ...interface{})
	Print(a ...interface{})

	Debugf(format string, a ...interface{})
	Debugln(a ...interface{})
	Debug(a ...interface{})
	Noticef(format string, a ...interface{})
	Noticeln(a ...interface{})
	Notice(a ...interface{})
	Infof(format string, a ...interface{})
	Infoln(a ...interface{})
	Info(a ...interface{})
	Successf(format string, a ...interface{})
	Successln(a ...interface{})
	Success(a ...interface{})
	Warnf(format string, a ...interface{})
	Warnln(a ...interface{})
	Warn(a ...interface{})
	Errorf(format string, a ...interface{})
	Errorln(a ...interface{})
	Error(a ...interface{})

	Level() LogLevel
	SetLevel(logLevel LogLevel)
	SetLevelString(logLevel string) error

	Spinner() Spinner
	Table() Table
	Section() Section
	ProgressBar() Progressbar
}

type Spinner interface {
	Start(text ...interface{}) (Spinner, error)
	Stop()
}

type Table interface {
	WithHasHeader(b ...bool) Table
	WithData(data [][]string) Table
	Srender() (string, error)
	Render() error
}

type Section interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}

type Progressbar interface {
	WithTitle(name string) Progressbar
	WithTotal(total int) Progressbar
	WithCurrent(current int) Progressbar
	WithBarCharacter(char string) Progressbar
	WithLastCharacter(char string) Progressbar
	WithElapsedTimeRoundingFactor(duration time.Duration) Progressbar
	WithShowElapsedTime(b ...bool) Progressbar
	WithShowCount(b ...bool) Progressbar
	WithShowTitle(b ...bool) Progressbar
	WithShowPercentage(b ...bool) Progressbar
	WithTitleStyle(style *pterm.Style) Progressbar
	WithBarStyle(style *pterm.Style) Progressbar
	WithRemoveWhenDone(b ...bool) Progressbar

	Increment()
	Add(count int)
	UpdateTitle(title string)
	Start() (Progressbar, error)
	Stop()
}
