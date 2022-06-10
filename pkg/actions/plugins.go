package actions

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/23doors/go-yaml/parser"
	"github.com/Masterminds/semver"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
)

type PluginManager struct {
	log      logger.Logger
	loader   *plugins.Loader
	cfg      *config.Project
	hostAddr string
}

func NewPluginManager(log logger.Logger, cfg *config.Project, loader *plugins.Loader, hostAddr string) *PluginManager {
	return &PluginManager{
		log:      log,
		cfg:      cfg,
		loader:   loader,
		hostAddr: hostAddr,
	}
}

func (m *PluginManager) isPluginsListInYAML() bool {
	return regexp.MustCompile(`((^|\n)plugins:\s*)`).Find(m.cfg.YAMLData()) != nil
}

func (m *PluginManager) insertInYAML(data []byte) error {
	fi, err := os.Stat(m.cfg.YAMLPath())
	if err != nil {
		return err
	}

	configData := m.cfg.YAMLData()
	result := append(configData, append([]byte("\n"), data...)...) //nolint: gocritic

	return fileutil.WriteFile(m.cfg.YAMLPath(), result, fi.Mode())
}

func (m *PluginManager) replacePluginsInYAML(replaced []byte) error {
	configData := m.cfg.YAMLData()

	fi, err := os.Stat(m.cfg.YAMLPath())
	if err != nil {
		return err
	}

	if !m.isPluginsListInYAML() {
		return merry.Errorf("plugins key not found in config yaml")
	}

	result := regexp.MustCompile(`((^|\n)plugins:\s*).*(\n\s+.*)*`).ReplaceAll(configData, append([]byte("$1"), bytes.TrimLeft(replaced, " ")...))

	return fileutil.WriteFile(m.cfg.YAMLPath(), result, fi.Mode())
}

func (m *PluginManager) deletePluginsInYAML() error {
	configData := m.cfg.YAMLData()

	fi, err := os.Stat(m.cfg.YAMLPath())
	if err != nil {
		return err
	}

	if !m.isPluginsListInYAML() {
		return merry.Errorf("plugins key not found in config yaml")
	}

	result := regexp.MustCompile(`(^|\n)plugins:\s*.*(\n\s+.*)*`).ReplaceAll(configData, []byte("$1"))

	return fileutil.WriteFile(m.cfg.YAMLPath(), result, fi.Mode())
}

func (m *PluginManager) Update(ctx context.Context, updateForce bool) error {
	prog, _ := m.log.ProgressBar().WithTotal(len(m.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()
	loadedPlugins := make([]*plugins.Plugin, len(m.cfg.Plugins))

	var updatedPlugins []*config.Plugin

	configData := m.cfg.YAMLData()
	configFile, _ := parser.ParseBytes(configData, parser.ParseComments)
	path, _ := yaml.PathString("$.plugins")
	pluginsNode, _ := path.FilterFile(configFile)

	for i, p := range m.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		cur := p.Loaded().Version
		verConstr := p.VerConstr()

		if updateForce {
			verConstr = nil
		}

		path, _ := yaml.PathString(fmt.Sprintf("$[%d].version", i))
		n, _ := path.FilterNode(pluginsNode)

		matching, _, err := m.loader.MatchingVersion(ctx, p.Name, p.Source, verConstr)
		if err != nil {
			return err
		}

		if updateForce && (p.VerConstr() == nil || !p.VerConstr().Check(matching)) {
			n.(*ast.StringNode).Value = fmt.Sprintf("^%s", matching.String())
		}

		if !matching.GreaterThan(cur) {
			loadedPlugins[i] = p.Loaded()

			prog.Increment()

			continue
		}

		matchingConstr, _ := semver.NewConstraint(matching.String())

		// Download new plugin version.
		plug, err := m.loader.DownloadPlugin(ctx, p.Name, matchingConstr, p.Source, nil)
		if err != nil {
			return err
		}

		updatedPlugins = append(updatedPlugins, p)
		loadedPlugins[i] = plug

		p.SetLoaded(plug)
		prog.Increment()
	}

	if updateForce && len(updatedPlugins) != 0 {
		replaced, _ := pluginsNode.MarshalYAML()

		err := m.replacePluginsInYAML(replaced)
		if err != nil {
			return merry.Errorf("error while updating config: %w", err)
		}
	}

	m.cfg.SetLoadedPlugins(loadedPlugins)

	// Print updated plugins info.
	if len(updatedPlugins) == 0 {
		m.log.Println("No updates found.")
	}

	for _, p := range updatedPlugins {
		m.log.Successf("Plugin '%s' updated to %s.\n", p.Name, pterm.Magenta(p.Loaded().Version.String()))
	}

	return nil
}

func (m *PluginManager) findPluginIndexByName(name string) int {
	for i, p := range m.cfg.Plugins {
		if strings.EqualFold(p.Name, name) {
			return i
		}
	}

	return -1
}

type PluginManagerAddOptions struct {
	Source  string
	Version string
}

func (m *PluginManager) Add(ctx context.Context, name string, opts *PluginManagerAddOptions) error {
	found := m.findPluginIndexByName(name)
	if found != -1 {
		return merry.Errorf("plugin with name: '%s' already defined", name)
	}

	plug := &config.Plugin{
		Name:    name,
		Source:  opts.Source,
		Version: opts.Version,
	}

	m.cfg.Plugins = append(m.cfg.Plugins, plug)

	err := m.cfg.LoadPlugins(ctx, m.log, m.loader, m.hostAddr)
	if err != nil {
		return err
	}

	var deployPlugins, runPlugins []string

	for _, p := range m.cfg.Plugins {
		if p.Loaded().HasAction(plugins.ActionDeploy) {
			deployPlugins = append(deployPlugins, p.Name)
		}

		if p.Loaded().HasAction(plugins.ActionRun) {
			runPlugins = append(runPlugins, p.Name)
		}
	}

	plug.Version = fmt.Sprintf("^%s", plug.Loaded().Version.String())

	initRes, err := plug.Loaded().Client().ProjectInit(ctx, m.cfg.Name, deployPlugins, runPlugins, m.cfg.Defaults.DNS.Plugin, nil)
	if err != nil {
		if st, ok := util.StatusFromError(err); ok && st.Code() == codes.Aborted {
			m.log.Println("Init canceled.")

			return nil
		}

		return err
	}

	configData := m.cfg.YAMLData()
	configFile, _ := parser.ParseBytes(configData, parser.ParseComments)

	plug.Other = initRes.Properties.AsMap()

	if m.isPluginsListInYAML() {
		path, _ := yaml.PathString("$.plugins")
		pluginsNode, _ := path.FilterFile(configFile)

		node, err := yaml.ValueToNode(plug)
		if err != nil {
			return merry.Errorf("error marshaling plugin: %w", err)
		}

		seqNode := pluginsNode.(*ast.SequenceNode)
		seqNode.Values = append(seqNode.Values, node)

		n, _ := pluginsNode.MarshalYAML()

		err = m.replacePluginsInYAML(n)
		if err != nil {
			return merry.Errorf("error while updating config: %w", err)
		}
	} else {
		pluginsYAML, err := yaml.Marshal(map[string]interface{}{"plugins": m.cfg.Plugins})
		if err != nil {
			return merry.Errorf("error marshaling plugin: %w", err)
		}

		err = m.insertInYAML(pluginsYAML)
		if err != nil {
			return merry.Errorf("error while updating config: %w", err)
		}
	}

	return nil
}

func (m *PluginManager) Delete(ctx context.Context, name string) error {
	found := m.findPluginIndexByName(name)

	if found == -1 {
		return merry.Errorf("plugin with name: '%s' not found", name)
	}

	configData := m.cfg.YAMLData()
	configFile, _ := parser.ParseBytes(configData, parser.ParseComments)
	path, _ := yaml.PathString("$.plugins")
	pluginsNode, _ := path.FilterFile(configFile)

	seqNode := pluginsNode.(*ast.SequenceNode)
	seqNode.Values = slices.Delete(seqNode.Values, found, found+1)

	replaced, _ := pluginsNode.MarshalYAML()

	var err error

	if len(seqNode.Values) >= 1 {
		err = m.replacePluginsInYAML(replaced)
	} else {
		err = m.deletePluginsInYAML()
	}

	if err != nil {
		return merry.Errorf("error while updating config: %w", err)
	}

	return nil
}

func (m *PluginManager) List(ctx context.Context) error {
	prog, _ := m.log.ProgressBar().WithTotal(len(m.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()

	data := [][]string{
		{"Name", "Range", "Current", "Wanted", "Latest"},
	}

	for _, p := range m.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		matching, latest, err := m.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerConstr())
		if err != nil {
			return err
		}

		cur := p.Loaded().Version

		matchingStr := matching.String()
		if matching.Equal(cur) {
			matchingStr = "-"
		}

		latestStr := latest.String()
		if latest.Equal(cur) {
			latestStr = "-"
		}

		data = append(data, []string{
			pterm.Yellow(p.Name),
			p.Version,
			p.Loaded().Version.String(),
			pterm.Green(matchingStr),
			pterm.Magenta(latestStr),
		})

		prog.Increment()
	}

	return m.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
}
