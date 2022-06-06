package util

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/google/shlex"
)

var (
	DefaultEditors    = []string{"vim", "nano", "vi"}
	ErrEditorNotFound = fmt.Errorf("error looking up editor! define 'EDITOR' environment variable")
)

func HashFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func LookupAnyPath(paths ...string) string {
	for _, p := range paths {
		path, err := exec.LookPath(p)
		if err == nil {
			return path
		}
	}

	return ""
}

func RunEditor(path string) error {
	var cmd *exec.Cmd

	editor := os.Getenv("EDITOR")

	if editor == "" {
		editor = LookupAnyPath("vim", "nano", "vi")
		if editor == "" {
			return ErrEditorNotFound
		}

		cmd = exec.Command(editor, path)
	} else {
		parts, err := shlex.Split(editor)
		if err != nil {
			return fmt.Errorf("invalid 'EDITOR' environment variable value: %s", editor)
		}

		parts = append(parts, path)
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
