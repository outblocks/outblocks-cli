package actions

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type SecretsManager struct {
	log logger.Logger
	cfg *config.Project
}

func NewSecretsManager(log logger.Logger, cfg *config.Project) *SecretsManager {
	return &SecretsManager{
		log: log,
		cfg: cfg,
	}
}

func (m *SecretsManager) init(ctx context.Context) error {
	plugin := m.cfg.Secrets.Plugin()
	if plugin == nil {
		return fmt.Errorf("secrets has no supported plugin available")
	}

	return plugin.Client().Start(ctx)
}

func (m *SecretsManager) View(ctx context.Context) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	m.log.Debugln("Loading secrets...")

	vals, err := m.getSecretsValues(ctx)
	if err != nil {
		return err
	}

	out, err := m.generateSecretsYAMLString(ctx, vals)
	if err != nil {
		return err
	}

	m.log.Successf("Secrets loaded!!\n")
	m.log.Println(out)

	return nil
}

func (m *SecretsManager) Get(ctx context.Context, key string) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	val, ok, err := m.cfg.Secrets.Plugin().Client().GetSecret(ctx, key, m.cfg.Secrets.Type, m.cfg.Secrets.Other)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("no secret found with key '%s'", key)
	}

	m.log.Successf("Secret value loaded!\n")

	s, _ := yaml.Marshal(map[string]string{key: val})

	m.log.Print(string(s))

	return nil
}

func (m *SecretsManager) Set(ctx context.Context, key, value string) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	changed, err := m.cfg.Secrets.Plugin().Client().SetSecret(ctx, key, value, m.cfg.Secrets.Type, m.cfg.Secrets.Other)
	if err != nil {
		return err
	}

	if changed {
		m.log.Successf("Secret value for '%s' set.\n", key)
	} else {
		m.log.Infof("Secret value for '%s' unchanged.\n", key)
	}

	return nil
}

func (m *SecretsManager) Delete(ctx context.Context, key string) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	deleted, err := m.cfg.Secrets.Plugin().Client().DeleteSecret(ctx, key, m.cfg.Secrets.Type, m.cfg.Secrets.Other)
	if err != nil {
		return err
	}

	if deleted {
		m.log.Successf("Deleted secret for '%s' key.\n", key)
	} else {
		m.log.Warnf("Secret for '%s' key not found.\n", key)
	}

	return nil
}

func (m *SecretsManager) getSecretsValues(ctx context.Context) (vals map[string]string, err error) {
	vals, err = m.cfg.Secrets.Plugin().Client().GetSecrets(ctx, m.cfg.Secrets.Type, m.cfg.Secrets.Other)
	if err != nil {
		return nil, err
	}

	for _, plug := range m.cfg.LoadedPlugins() {
		for k := range plug.Secrets {
			if _, ok := vals[k]; !ok {
				vals[k] = ""
			}
		}
	}

	return vals, nil
}

func (m *SecretsManager) generateSecretsYAMLString(ctx context.Context, vals map[string]string) (string, error) {
	var (
		ret              []string
		pluginSecretKeys []string
	)

	pluginSecrets := make(map[string][]*plugins.PluginSecret)

	for _, plug := range m.cfg.LoadedPlugins() {
		for k, v := range plug.Secrets {
			if _, ok := pluginSecrets[k]; !ok {
				pluginSecretKeys = append(pluginSecretKeys, k)
			}

			pluginSecrets[k] = append(pluginSecrets[k], v)
		}
	}

	for k := range pluginSecrets {
		if _, ok := vals[k]; !ok {
			vals[k] = ""
		}
	}

	keys := make([]string, 0, len(vals))

	for k := range vals {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range pluginSecretKeys {
		for _, ps := range pluginSecrets[k] {
			if ps.Description == "" {
				continue
			}

			ret = append(ret, fmt.Sprintf("# %s", ps.Description))
		}

		s, _ := yaml.Marshal(map[string]string{k: vals[k]})
		ret = append(ret, strings.TrimRight(string(s), "\n"))
	}

	if len(pluginSecrets) > 0 {
		ret = append(ret, "")
	}

	for _, k := range keys {
		if pluginSecrets[k] != nil {
			continue
		}

		s, _ := yaml.MarshalWithOptions(map[string]string{k: vals[k]})
		ret = append(ret, strings.TrimRight(string(s), "\n"))
	}

	return strings.Join(ret, "\n"), nil
}

func (m *SecretsManager) Edit(ctx context.Context) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	m.log.Debugln("Loading secrets...")

	vals, err := m.getSecretsValues(ctx)
	if err != nil {
		return err
	}

	out, err := m.generateSecretsYAMLString(ctx, vals)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp("", fmt.Sprintf("secrets-%s-%s-", m.cfg.Name, m.cfg.Env()))
	if err != nil {
		return err
	}

	if _, err = f.WriteString(out); err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	for {
		err = util.RunEditor(f.Name())
		if err != nil {
			return merry.Errorf("run editor error: %w", err)
		}

		data, err := ioutil.ReadFile(f.Name())
		if err != nil {
			return merry.Errorf("loading edited file error: %w", err)
		}

		res := make(map[string]string)

		err = yaml.Unmarshal(data, &res)
		if err != nil {
			m.log.Errorf("Loading edited file error:\n%s\nPress a key to return to the editor or Ctrl+C to exit.\n", err)

			ch := make(chan struct{})

			go func() {
				bufio.NewReader(os.Stdin).ReadByte()
				close(ch)
			}()

			select {
			case <-ch:
			case <-ctx.Done():
				return ctx.Err()
			}

			continue
		}

		d, err := statefile.NewMapDiff(vals, res, 1)
		if err != nil {
			return merry.Errorf("computing changes error: %w", err)
		}

		if d.IsEmpty() {
			m.log.Println("No changes detected.")

			return nil
		}

		m.log.Infoln("Saving secrets...")
		err = m.cfg.Secrets.Plugin().Client().ReplaceSecrets(ctx, res, m.cfg.Secrets.Type, m.cfg.Secrets.Other)

		return err
	}
}

func (m *SecretsManager) Import(ctx context.Context, f string) error {
	err := m.init(ctx)
	if err != nil {
		return err
	}

	m.log.Debugln("Loading secrets...")

	vals, err := m.getSecretsValues(ctx)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(f)
	if err != nil {
		return merry.Errorf("loading import file error: %w", err)
	}

	res := make(map[string]string)

	err = yaml.Unmarshal(data, &res)
	if err != nil {
		return merry.Errorf("loading import file error:\n%w", err)
	}

	d, err := statefile.NewMapDiff(vals, res, 1)
	if err != nil {
		return merry.Errorf("computing changes error: %w", err)
	}

	if d.IsEmpty() {
		m.log.Println("No changes detected.")

		return nil
	}

	m.log.Infoln("Saving secrets...")
	err = m.cfg.Secrets.Plugin().Client().ReplaceSecrets(ctx, res, m.cfg.Secrets.Type, m.cfg.Secrets.Other)

	return err
}
