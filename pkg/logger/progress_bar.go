package logger

import (
	"os"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

type ProgressbarPrinter struct {
	*pterm.ProgressbarPrinter

	m *sync.Mutex
}

// WithTitle sets the name of the ProgressbarPrinter.
func (p ProgressbarPrinter) WithTitle(name string) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTitle(name),
	}
}

// // WithTotal sets the total value of the ProgressbarPrinter.
func (p ProgressbarPrinter) WithTotal(total int) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTotal(total),
	}
}

// WithCurrent sets the current value of the ProgressbarPrinter.
func (p ProgressbarPrinter) WithCurrent(current int) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithCurrent(current),
	}
}

// WithBarCharacter sets the bar character of the ProgressbarPrinter.
func (p ProgressbarPrinter) WithBarCharacter(char string) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithBarCharacter(char),
	}
}

// WithLastCharacter sets the last character of the ProgressbarPrinter.
func (p ProgressbarPrinter) WithLastCharacter(char string) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithLastCharacter(char),
	}
}

// WithElapsedTimeRoundingFactor sets the rounding factor of the elapsed time.
func (p ProgressbarPrinter) WithElapsedTimeRoundingFactor(duration time.Duration) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithElapsedTimeRoundingFactor(duration),
	}
}

// WithShowElapsedTime sets if the elapsed time should be displayed in the ProgressbarPrinter.
func (p ProgressbarPrinter) WithShowElapsedTime(b ...bool) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowElapsedTime(b...),
	}
}

// WithShowCount sets if the total and current count should be displayed in the ProgressbarPrinter.
func (p ProgressbarPrinter) WithShowCount(b ...bool) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowCount(b...),
	}
}

// WithShowTitle sets if the title should be displayed in the ProgressbarPrinter.
func (p ProgressbarPrinter) WithShowTitle(b ...bool) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowTitle(b...),
	}
}

// WithShowPercentage sets if the completed percentage should be displayed in the ProgressbarPrinter.
func (p ProgressbarPrinter) WithShowPercentage(b ...bool) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowPercentage(b...),
	}
}

// WithTitleStyle sets the style of the title.
func (p ProgressbarPrinter) WithTitleStyle(style *pterm.Style) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTitleStyle(style),
	}
}

// WithBarStyle sets the style of the bar.
func (p ProgressbarPrinter) WithBarStyle(style *pterm.Style) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithBarStyle(style),
	}
}

// WithRemoveWhenDone sets if the ProgressbarPrinter should be removed when it is done.
func (p ProgressbarPrinter) WithRemoveWhenDone(b ...bool) *ProgressbarPrinter {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithRemoveWhenDone(b...),
	}
}

// Increment current value by one.
func (p *ProgressbarPrinter) Increment() {
	p.m.Lock()
	defer p.m.Unlock()

	p.ProgressbarPrinter.Increment()
}

// Add to current value.
func (p *ProgressbarPrinter) Add(count int) {
	p.m.Lock()
	defer p.m.Unlock()

	p.ProgressbarPrinter.Add(count)
}

// Add to current value.
func (p *ProgressbarPrinter) UpdateTitle(title string) {
	p.m.Lock()
	defer p.m.Unlock()

	p.ProgressbarPrinter.Title = title
	p.ProgressbarPrinter.Add(0)
}

func (p *ProgressbarPrinter) Start() (*ProgressbarPrinter, error) {
	started, err := p.ProgressbarPrinter.Start()
	if err != nil {
		return nil, err
	}

	pp := &ProgressbarPrinter{
		ProgressbarPrinter: started,
		m:                  &sync.Mutex{},
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return pp, nil
	}

	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()

		for {
			<-t.C
			pp.m.Lock()

			if !started.IsActive {
				pp.m.Unlock()

				return
			}

			started.Add(0)
			pp.m.Unlock()
		}
	}()

	return pp, nil
}
