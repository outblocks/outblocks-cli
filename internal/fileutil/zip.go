package fileutil

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/gobwas/glob"
)

func CheckMatch(name string, globs []glob.Glob) bool {
	for _, g := range globs {
		if g.Match(name) {
			return true
		}
	}

	return false
}

func ArchiveDir(dir, out string, excludes []string) error {
	for i := range excludes {
		excludes[i] = filepath.FromSlash(excludes[i])
	}

	var (
		g                          glob.Glob
		excludeGlobs, includeGlobs []glob.Glob
		err                        error
	)

	for _, pat := range excludes {
		if len(pat) > 0 && pat[0] == '!' {
			g, err = glob.Compile(pat[1:])
			if err != nil {
				return merry.Errorf("unable to parse inclusion '%s': %w", pat, err)
			}

			excludeGlobs = append(excludeGlobs, g)
		} else {
			g, err = glob.Compile(pat)
			if err != nil {
				return merry.Errorf("unable to parse exclusion '%s': %w", pat, err)
			}

			includeGlobs = append(includeGlobs, g)
		}
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}

	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error encountered during file walk: %s", err)
		}

		relname, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("error relativizing file for archival: %s", err)
		}

		isMatch := CheckMatch(relname, excludeGlobs)
		if isMatch && CheckMatch(relname, includeGlobs) {
			isMatch = false
		}

		if info.IsDir() {
			if isMatch {
				return filepath.SkipDir
			}
			return nil
		}

		if isMatch {
			return nil
		}

		if err != nil {
			return err
		}

		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("error creating file header: %s", err)
		}
		fh.Name = filepath.ToSlash(relname)
		fh.Method = zip.Deflate
		// fh.Modified alone isn't enough when using a zero value
		fh.SetModTime(time.Time{}) //nolint

		f, err := w.CreateHeader(fh)
		if err != nil {
			return fmt.Errorf("error creating file inside archive: %s", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file for archival: %s", err)
		}
		_, err = f.Write(content)
		return err
	})
}
