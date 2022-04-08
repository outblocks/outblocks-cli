package config

import (
	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

const (
	AppTypeFunction = "function"
)

type FunctionApp struct {
	BasicApp `json:",inline"`
}

func LoadFunctionAppData(path string, n ast.Node) (App, error) {
	out := &FunctionApp{
		BasicApp: BasicApp{
			AppRun:    &AppRunInfo{},
			AppDeploy: &AppDeployInfo{},
		},
	}

	if err := util.YAMLNodeDecode(n, out); err != nil {
		return nil, merry.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = []byte(n.String())

	return out, nil
}

func (s *FunctionApp) SupportsLocal() bool {
	return false
}

func (s *FunctionApp) BuildProto() *apiv1.AppBuild {
	return nil
}
