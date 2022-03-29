package statefile

import (
	"encoding/json"

	"github.com/ansel1/merry/v2"
)

const latestStateVersion = 1

type stateVersionInfo struct {
	Version int `json:"version"`
}

func ReadState(in []byte) (*StateData, error) {
	if len(in) == 0 {
		return NewStateData(), nil
	}

	versionInfo := &stateVersionInfo{}

	err := json.Unmarshal(in, &versionInfo)
	if err != nil {
		return nil, merry.Errorf("error reading state info: %w", err)
	}

	out := NewStateData()

	err = json.Unmarshal(in, out)

	return out, err
}
