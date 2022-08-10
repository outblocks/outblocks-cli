package util

import (
	"strings"

	"github.com/ansel1/merry/v2"
	"golang.org/x/exp/slices"
)

type MatchType int

const (
	AppTypeService  = "service"
	AppTypeFunction = "function"
	AppTypeStatic   = "static"
)

const (
	MatchTypeUnknown MatchType = iota
	MatchTypeAny
	MatchTypeAppService
	MatchTypeAppFunction
	MatchTypeAppStatic
	MatchTypeAppAny
	MatchTypeDependency
)

func (m MatchType) IsApp() bool {
	return m <= MatchTypeAppAny && m > MatchTypeAny
}

func (m MatchType) IsDependency() bool {
	return m == MatchTypeDependency
}

type TargetMatch struct {
	input   string
	typ     MatchType
	name    string
	matched int
}

func (t *TargetMatch) Input() string {
	return t.input
}

type TargetMatcher struct {
	matchers []*TargetMatch
}

func NewTargetMatcher() *TargetMatcher {
	return &TargetMatcher{}
}

func (m *TargetMatcher) ResetMatches() {
	for _, t := range m.matchers {
		t.matched = 0
	}
}

func (m *TargetMatcher) idSplit(id string) (typ MatchType, name string) {
	tsplit := strings.SplitN(id, "_", 3)

	if len(tsplit) == 3 && tsplit[0] == "app" {
		switch tsplit[1] {
		case AppTypeService:
			typ = MatchTypeAppService
		case AppTypeFunction:
			typ = MatchTypeAppFunction
		case AppTypeStatic:
			typ = MatchTypeAppStatic
		}

		return typ, tsplit[2]
	}

	tsplit = strings.SplitN(id, "_", 2)

	if len(tsplit) == 2 && tsplit[0] == "dep" {
		return MatchTypeDependency, tsplit[1]
	}

	return MatchTypeUnknown, ""
}

func (m *TargetMatcher) add(name string, defaultTyp MatchType, allowApp, allowDep bool) error {
	typ := defaultTyp

	tsplit := strings.SplitN(name, ".", 2)
	if len(tsplit) > 2 {
		return merry.Errorf("wrong format for target '%s': specify in a form of <app type>.<name>, e.g.: static.website", name)
	}

	if len(tsplit) == 2 {
		switch strings.ToLower(tsplit[0]) {
		case AppTypeService:
			typ = MatchTypeAppService
		case AppTypeFunction:
			typ = MatchTypeAppFunction
		case AppTypeStatic:
			typ = MatchTypeAppStatic
		case "dependency", "dep":
			typ = MatchTypeDependency
		}

		if !allowDep && typ.IsDependency() {
			return merry.Errorf("dependency '%s' not allowed as a target here", tsplit[1])
		}

		if !allowApp && typ.IsApp() {
			return merry.Errorf("app '%s' not allowed as a target here", tsplit[1])
		}

		m.matchers = append(m.matchers, &TargetMatch{
			input: name,
			typ:   typ,
			name:  tsplit[1],
		})

		return nil
	}

	// ID mode.
	typ, n := m.idSplit(name)
	if typ != MatchTypeUnknown {
		if !allowDep && typ.IsDependency() {
			return merry.Errorf("dependency '%s' not allowed as a target here", n)
		}

		if !allowApp && typ.IsApp() {
			return merry.Errorf("app '%s' not allowed as a target here", n)
		}

		m.matchers = append(m.matchers, &TargetMatch{
			input: name,
			typ:   typ,
			name:  n,
		})

		return nil
	}

	m.matchers = append(m.matchers, &TargetMatch{
		input: name,
		typ:   defaultTyp,
		name:  name,
	})

	return nil
}

func (m *TargetMatcher) Add(name string) error {
	return m.add(name, MatchTypeAny, true, true)
}

func (m *TargetMatcher) AddApp(name string) error {
	return m.add(name, MatchTypeAppAny, true, false)
}

func (m *TargetMatcher) AddDependency(name string) error {
	return m.add(name, MatchTypeDependency, false, true)
}

func (m *TargetMatcher) IsEmpty() bool {
	return m == nil || len(m.matchers) == 0
}

func (m *TargetMatcher) findByID(id string) int {
	typ, name := m.idSplit(id)

	if typ == MatchTypeUnknown {
		return -1
	}

	for i, m := range m.matchers {
		if m.name != name {
			continue
		}

		if m.typ == MatchTypeAny || m.typ == typ || (m.typ == MatchTypeAppAny && typ.IsApp()) {
			return i
		}
	}

	return -1
}

func (m *TargetMatcher) Matches(id string) bool {
	if m == nil {
		return false
	}

	match := m.findByID(id)

	if match != -1 {
		m.matchers[match].matched++

		return true
	}

	return false
}

func (m *TargetMatcher) Unmatched() []*TargetMatch {
	if m == nil {
		return nil
	}

	var ret []*TargetMatch

	for _, v := range m.matchers {
		if v.matched == 0 {
			ret = append(ret, v)
		}
	}

	return ret
}

func (m *TargetMatcher) MatchedMoreThanOnce() []*TargetMatch {
	if m == nil {
		return nil
	}

	var ret []*TargetMatch

	for _, v := range m.matchers {
		if v.matched > 1 {
			ret = append(ret, v)
		}
	}

	return ret
}

func (m *TargetMatcher) Remove(id string) bool {
	match := m.findByID(id)

	if match != -1 {
		m.matchers = slices.Delete(m.matchers, match, match)

		return true
	}

	return false
}
