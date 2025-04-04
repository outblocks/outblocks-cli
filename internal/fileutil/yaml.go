package fileutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/ansel1/merry/v2"
	"github.com/enescakir/emoji"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func FindYAMLGoingUp(path, name string) string {
	var f string

	for {
		f = FindYAML(filepath.Join(path, name))
		if f != "" {
			break
		}

		newpath := filepath.Dir(path)
		if newpath == path {
			break
		}

		path = newpath
	}

	return f
}

func FindYAML(path string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		f := path + ext

		if plugin_util.FileExists(f) {
			return f
		}
	}

	return ""
}

func FindMatchingBaseDir(path string, matching []string, stopDir string) string {
	for {
		if path == stopDir {
			break
		}

		base := filepath.Base(path)
		for _, m := range matching {
			if base == m {
				return path
			}
		}

		newpath := filepath.Dir(path)
		if newpath == path {
			break
		}

		path = newpath
	}

	return ""
}

func FindSubdirsOfMatching(path string, matching []string) []string {
	var dirs []string

	_ = filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				p := filepath.Base(path)

				for _, m := range matching {
					if m == p {
						dirs = append(dirs, path)
					}
				}
			}

			return nil
		})

	return dirs
}

func FindYAMLFilesByName(path string, name ...string) []string {
	return FindYAMLFiles(path, func(f string) bool {
		for _, ext := range []string{".yaml", ".yml"} {
			for _, n := range name {
				if n+ext != f {
					continue
				}

				return true
			}
		}

		return false
	})
}

func FindYAMLFiles(path string, match func(string) bool) []string {
	var files []string

	pathMap := make(map[string]struct{})

	_ = filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if _, ok := pathMap[path]; ok {
				return nil
			}

			if !match(info.Name()) {
				return nil
			}

			files = append(files, path)
			pathMap[path] = struct{}{}

			return nil
		})

	return files
}

func YAMLError(path, msg string, data []byte) error {
	yamlPath, err := yaml.PathString(path)
	if err != nil {
		panic(err)
	}

	source, err := yamlPath.AnnotateSourceDefault(data)
	if err != nil {
		idx := strings.LastIndex(path, ".")
		if idx == -1 {
			return merry.Errorf("\n%s", msg)
		}

		return YAMLError(path[:idx], msg, data)
	}

	return merry.Errorf("\n%s\n\n%s  %s", source, emoji.Warning, msg)
}

func IsRelativeSubdir(parent, dir string) bool {
	parent, _ = filepath.Abs(parent)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(parent, dir)
	}

	rel, err := filepath.Rel(parent, dir)

	return err == nil && !strings.HasPrefix(rel, "..")
}
