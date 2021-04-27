package config_test

import (
	"strings"
	"testing"

	"github.com/outblocks/outblocks-cli/pkg/config"
)

func TestExpand(t *testing.T) {
	tests := []struct {
		content  string
		vars     map[string]interface{}
		expected string
	}{
		{
			content:  "abc: ${var.abc}",
			vars:     map[string]interface{}{"var": map[string]interface{}{"abc": 1}},
			expected: `abc: 1`,
		},
		{
			content:  "abc: ${var}\na: true",
			vars:     map[string]interface{}{"var": map[string]interface{}{"abc": 1, "cba": []interface{}{"1", 2, true, 1.5}}},
			expected: "abc: {abc: 1, cba: [\"1\", 2, true, 1.5]}\na: true",
		},
		{
			content:  "abc: I am ${var}",
			vars:     map[string]interface{}{"var": "cornholio"},
			expected: "abc: I am cornholio",
		},
		{
			content:  "abc: ${var.nested.val.y}",
			vars:     map[string]interface{}{"var": map[string]interface{}{"nested": map[string]interface{}{"val": map[string]interface{}{"y": 1}}}},
			expected: `abc: 1`,
		},
	}

	for _, test := range tests {
		out, err := config.NewYAMLEvaluator(test.vars).Expand([]byte(test.content))
		if err != nil {
			t.Fatalf(`Expand(%q) for %q = (%q, %q), expected non error`, test.content, test.vars, out, err)
		}
		if test.expected != string(out) {
			t.Fatalf(`Expand(%q) for %q = (%q, %q), expected: %q`, test.content, test.vars, out, err, test.expected)
		}
	}
}

func TestExpand_Invalid(t *testing.T) {
	tests := []struct {
		content  string
		vars     map[string]interface{}
		expected string
	}{
		{
			content:  "\nabc ${}",
			vars:     nil,
			expected: "[2:5] empty expansion found",
		},
		{
			content:  "abc ${var.abc}",
			vars:     nil,
			expected: "[1:5] expansion value 'var.abc' could not be evaluated",
		},
	}

	for _, test := range tests {
		out, err := config.NewYAMLEvaluator(test.vars).Expand([]byte(test.content))
		if err == nil {
			t.Fatalf(`Expand(%q) for %q = (%q, %q), expected error`, test.content, test.vars, out, err)
		}
		if !strings.Contains(err.Error(), test.expected) {
			t.Fatalf(`Expand(%q) for %q = (%q, %q), expected error: %q`, test.content, test.vars, out, err, test.expected)
		}
	}
}
