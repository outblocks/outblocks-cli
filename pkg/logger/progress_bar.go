package logger

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/pterm/pterm"
)

type ProgressbarPrinter struct {
	*pterm.ProgressbarPrinter

	m *sync.Mutex
}

// WithTitle sets the name of the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithTitle(name string) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTitle(name),
	}
}

// // WithTotal sets the total value of the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithTotal(total int) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTotal(total),
	}
}

// WithCurrent sets the current value of the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithCurrent(current int) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithCurrent(current),
	}
}

// WithBarCharacter sets the bar character of the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithBarCharacter(char string) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithBarCharacter(char),
	}
}

// WithLastCharacter sets the last character of the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithLastCharacter(char string) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithLastCharacter(char),
	}
}

// WithElapsedTimeRoundingFactor sets the rounding factor of the elapsed time.
func (p *ProgressbarPrinter) WithElapsedTimeRoundingFactor(duration time.Duration) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithElapsedTimeRoundingFactor(duration),
	}
}

// WithShowElapsedTime sets if the elapsed time should be displayed in the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithShowElapsedTime(b ...bool) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowElapsedTime(b...),
	}
}

// WithShowCount sets if the total and current count should be displayed in the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithShowCount(b ...bool) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowCount(b...),
	}
}

// WithShowTitle sets if the title should be displayed in the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithShowTitle(b ...bool) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowTitle(b...),
	}
}

// WithShowPercentage sets if the completed percentage should be displayed in the ProgressbarPrinter.
func (p *ProgressbarPrinter) WithShowPercentage(b ...bool) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithShowPercentage(b...),
	}
}

// WithTitleStyle sets the style of the title.
func (p *ProgressbarPrinter) WithTitleStyle(style *pterm.Style) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithTitleStyle(style),
	}
}

// WithBarStyle sets the style of the bar.
func (p *ProgressbarPrinter) WithBarStyle(style *pterm.Style) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithBarStyle(style),
	}
}

// WithRemoveWhenDone sets if the ProgressbarPrinter should be removed when it is done.
func (p *ProgressbarPrinter) WithRemoveWhenDone(b ...bool) Progressbar {
	return &ProgressbarPrinter{
		ProgressbarPrinter: p.ProgressbarPrinter.WithRemoveWhenDone(b...),
	}
}

func (p *ProgressbarPrinter) updateProgressRaw() {
	if p.TitleStyle == nil {
		p.TitleStyle = pterm.NewStyle()
	}

	if p.BarStyle == nil {
		p.BarStyle = pterm.NewStyle()
	}

	if p.Total == 0 {
		return
	}

	var before, after string

	width := pterm.GetTerminalWidth()
	currentPercentage := int(math.Round(float64(int64(p.Current)) / float64(int64(p.Total)) * 100))

	decoratorCount := pterm.Gray("[") + pterm.LightWhite(p.Current) + pterm.Gray("/") + pterm.LightWhite(p.Total) + pterm.Gray("]")

	decoratorCurrentPercentage := color.RGB(pterm.NewRGB(255, 0, 0).Fade(0, float32(p.Total), float32(p.Current), pterm.NewRGB(0, 255, 0)).GetValues()).
		Sprint(strconv.Itoa(currentPercentage) + "%")

	decoratorTitle := p.TitleStyle.Sprint(p.Title)

	if p.ShowTitle {
		before += decoratorTitle + " "
	}

	if p.ShowCount {
		before += decoratorCount + " "
	}

	after += " "

	if p.ShowPercentage {
		after += decoratorCurrentPercentage + " "
	}

	barMaxLength := width - len(pterm.RemoveColorFromString(before)) - len(pterm.RemoveColorFromString(after)) - 1
	barCurrentLength := (p.Current * barMaxLength) / p.Total
	barFiller := strings.Repeat(p.BarFiller, barMaxLength-barCurrentLength)

	bar := p.BarStyle.Sprint(strings.Repeat(p.BarCharacter, barCurrentLength)+p.LastCharacter) + barFiller
	printer(pterm.Sprintln(before + bar + after))
}

// Increment current value by one.
func (p *ProgressbarPrinter) Increment() {
	p.Add(1)
}

// Add to current value.
func (p *ProgressbarPrinter) Add(count int) {
	p.m.Lock()
	defer p.m.Unlock()

	if util.IsTermDumb() {
		if p.Total == 0 {
			return
		}

		p.Current += count

		if p.Current >= p.Total {
			return
		}

		p.updateProgressRaw()

		return
	}

	p.ProgressbarPrinter.Add(count)
}

// Add to current value.
func (p *ProgressbarPrinter) UpdateTitle(title string) {
	p.m.Lock()
	defer p.m.Unlock()

	if util.IsTermDumb() {
		p.Title = title
		p.updateProgressRaw()

		return
	}

	p.ProgressbarPrinter.UpdateTitle(title)
}

func (p *ProgressbarPrinter) Start() (Progressbar, error) {
	if util.IsTermDumb() {
		p.ShowElapsedTime = false
		p.RemoveWhenDone = false

		pp := &ProgressbarPrinter{
			ProgressbarPrinter: p.ProgressbarPrinter,
			m:                  &sync.Mutex{},
		}

		pp.updateProgressRaw()

		return pp, nil
	}

	started, err := p.ProgressbarPrinter.Start()
	if err != nil {
		return nil, err
	}

	pp := &ProgressbarPrinter{
		ProgressbarPrinter: started,
		m:                  &sync.Mutex{},
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

func (p *ProgressbarPrinter) Stop() {
	_, _ = p.ProgressbarPrinter.Stop()
}
