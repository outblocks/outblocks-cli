package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	EnvPrefix = "OUTBLOCKS"
)

type Environment struct {
	v *viper.Viper

	vars []*EnvVar
}

type EnvVar struct {
	key         string
	description string
	def         string
}

func NewEnvironment(v *viper.Viper) *Environment {
	return &Environment{v: v}
}

func envName(key string) string {
	key = strings.NewReplacer(".", "_").Replace(key)
	key = strings.ToUpper("$" + EnvPrefix + "_" + key)

	return key
}

func (e *Environment) AddVar(key, description string) {
	e.AddVarWithDefault(key, description, "")
}

func (e *Environment) AddVarWithDefault(key, description, def string) {
	if def != "" {
		e.v.SetDefault(key, def)
	}

	e.vars = append(e.vars, &EnvVar{key: key, description: description, def: def})
}

func (e *Environment) Info() [][]string {
	info := make([][]string, len(e.vars))

	for i, v := range e.vars {
		info[i] = v.Info()
	}

	return info
}

func (v *EnvVar) Info() []string {
	def := ""

	if v.def != "" {
		def = fmt.Sprintf(" (default %s)", v.def)
	}

	return []string{envName(v.key), fmt.Sprintf("%s%s.", v.description, def)}
}

func (e *Environment) BindCLIFlag(key string, f *pflag.Flag) {
	err := e.v.BindPFlag(key, f)
	if err != nil {
		panic(err)
	}

	key = strings.NewReplacer(".", "_").Replace(key)
	key = strings.ToUpper(EnvPrefix + "_" + key)

	f.Usage += fmt.Sprintf(` [$%s]`, key)
}
