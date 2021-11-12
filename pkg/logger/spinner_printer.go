package logger

import (
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/pterm/pterm"
)

type SpinnerPrinter struct {
	pterm.SpinnerPrinter
}

func (s *SpinnerPrinter) Start(text ...interface{}) (Spinner, error) {
	if util.IsTermDumb() {
		if len(text) != 0 {
			s.Text = pterm.Sprint(text...)
		}

		printer(pterm.Sprintln(s.Text))

		started := *s
		started.RemoveWhenDone = false

		return &started, nil
	}

	started, err := s.SpinnerPrinter.Start(text...)

	return &SpinnerPrinter{
		SpinnerPrinter: *started,
	}, err
}

func (s *SpinnerPrinter) Stop() {
	if util.IsTermDumb() {
		return
	}

	_ = s.SpinnerPrinter.Stop()
}
