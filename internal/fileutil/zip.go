package fileutil

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"time"

	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func ArchiveDir(dir, out string, excludes []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}

	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return plugin_util.WalkWithExclusions(dir, excludes, func(path, rel string, info os.FileInfo) error {
		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("error creating file header: %s", err)
		}
		fh.Name = filepath.ToSlash(rel)
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
