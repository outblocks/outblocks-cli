package logger

import (
	"github.com/pterm/pterm"
)

type TablePrinter struct {
	*pterm.TablePrinter
}

func (p *TablePrinter) WithHasHeader(b ...bool) Table {
	return &TablePrinter{
		TablePrinter: p.TablePrinter.WithHasHeader(b...),
	}
}

func (p *TablePrinter) WithData(data [][]string) Table {
	return &TablePrinter{
		TablePrinter: p.TablePrinter.WithData(data),
	}
}

func (p *TablePrinter) Srender() (string, error) {
	return p.TablePrinter.Srender()
}

func (p *TablePrinter) Render() error {
	o, err := p.Srender()
	if err != nil {
		return err
	}

	printer(pterm.Sprintln(o))

	return nil
}
