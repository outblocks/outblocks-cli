package config

import (
	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	AppTypeFunction = "function"
)

type FunctionAppPackage struct {
	Patterns []string `json:"patterns"`
}

type FunctionApp struct {
	BasicApp                    `json:",inline"`
	types.FunctionAppProperties `json:",inline"`
	Package                     *FunctionAppPackage `json:"package"`

	AppBuild *apiv1.AppBuild `json:"-"`
}

func LoadFunctionAppData(projectName, path string, n ast.Node) (App, error) {
	out := &FunctionApp{
		BasicApp:              *NewBasicApp(),
		FunctionAppProperties: types.FunctionAppProperties{},
		AppBuild:              &apiv1.AppBuild{},
		Package:               &FunctionAppPackage{},
	}

	if err := util.YAMLNodeDecode(n, out); err != nil {
		return nil, merry.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	if out.Entrypoint == "" {
		out.Entrypoint = out.Name()
	}

	out.yamlPath = path
	out.yamlData = []byte(n.String())

	return out, nil
}

func (s *FunctionApp) SupportsLocal() bool {
	return true
}

func (s *FunctionApp) Proto() *apiv1.App {
	base := s.BasicApp.Proto()

	props, err := s.FunctionAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	mergedProps := plugin_util.MergeMaps(base.Properties.AsMap(), props)
	base.Properties = plugin_util.MustNewStruct(mergedProps)

	return base
}

func (s *FunctionApp) BuildProto() *apiv1.AppBuild {
	return s.AppBuild
}
