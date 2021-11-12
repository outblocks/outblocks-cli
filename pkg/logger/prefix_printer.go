package logger

import (
	"github.com/pterm/pterm"
)

type PrefixPrinter struct {
	*pterm.PrefixPrinter
}

func (p *PrefixPrinter) print(msg string) {
	if p.Debugger && !pterm.PrintDebugMessages {
		return
	}

	printer(msg)
}

func (p *PrefixPrinter) Println(a ...interface{}) {
	p.print(p.Sprintln(a...))
}

func (p *PrefixPrinter) Printf(format string, a ...interface{}) {
	p.print(p.Sprintf(format, a...))
}
