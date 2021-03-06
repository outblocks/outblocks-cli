package config

import (
	"fmt"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	AppTypeService = "service"
)

type ServiceApp struct {
	BasicApp                   `json:",inline"`
	types.ServiceAppProperties `json:",inline"`

	AppBuild *apiv1.AppBuild `json:"-"`
}

func LoadServiceAppData(projectName, path string, n ast.Node) (App, error) {
	out := &ServiceApp{
		BasicApp: *NewBasicApp(),
		ServiceAppProperties: types.ServiceAppProperties{
			Build: &types.ServiceAppBuild{
				Dockerfile: "Dockerfile",
			},
		},
		AppBuild: &apiv1.AppBuild{},
	}

	if err := util.YAMLNodeDecode(n, out); err != nil {
		return nil, merry.Errorf("load service config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.AppBuild.LocalDockerImage = fmt.Sprintf("outblocks/%s/%s", projectName, out.ID())
	if out.ServiceAppProperties.Build.DockerImage != "" {
		out.AppBuild.LocalDockerImage = out.ServiceAppProperties.Build.DockerImage
	}

	out.yamlPath = path
	out.yamlData = []byte(n.String())

	return out, nil
}

func (s *ServiceApp) SupportsLocal() bool {
	return true
}

func (s *ServiceApp) Proto() *apiv1.App {
	base := s.BasicApp.Proto()

	props, err := s.ServiceAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	mergedProps := plugin_util.MergeMaps(base.Properties.AsMap(), props)
	base.Properties = plugin_util.MustNewStruct(mergedProps)

	return base
}

func (s *ServiceApp) BuildProto() *apiv1.AppBuild {
	return s.AppBuild
}
