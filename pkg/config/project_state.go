package config

type ProjectState struct {
	Type  string                 `json:"type"`
	Other map[string]interface{} `yaml:"-,remain"`
}
