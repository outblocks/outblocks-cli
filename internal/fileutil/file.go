package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enescakir/emoji"
	"github.com/goccy/go-yaml"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

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

		if FileExists(f) {
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

func FindYAMLFiles(path, name string) []string {
	var files []string

	_ = filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			for _, ext := range []string{".yaml", ".yml"} {
				if name+ext == info.Name() {
					files = append(files, path)
					return nil
				}
			}

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
			panic(err)
		}

		return YAMLError(path[:idx], msg, data)
	}

	return fmt.Errorf("\n%s\n\n%s  %s", source, emoji.Warning, msg)
}
