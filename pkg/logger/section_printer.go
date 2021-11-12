package logger

import (
	"github.com/pterm/pterm"
)

type SectionPrinter struct {
	pterm.SectionPrinter
}

func (p *SectionPrinter) Printf(format string, a ...interface{}) {
	printer(p.Sprintf(format, a...))
}

func (p *SectionPrinter) Println(a ...interface{}) {
	printer(p.Sprintln(a...))
}
