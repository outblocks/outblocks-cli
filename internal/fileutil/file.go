package fileutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

func ChownToUser(path string) error {
	if runtime.GOOS == GOOSWindows || os.Geteuid() != 0 {
		return nil
	}

	uidStr, ok1 := os.LookupEnv("SUDO_UID")
	gidStr, ok2 := os.LookupEnv("SUDO_GID")

	if ok1 && ok2 {
		uid, _ := strconv.Atoi(uidStr)
		gid, _ := strconv.Atoi(gidStr)

		return os.Chown(path, uid, gid)
	}

	return nil
}

func ChownRToUser(path string) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = ChownToUser(name)
		}
		return err
	})
}

func LchownToUser(path string) error {
	if runtime.GOOS == GOOSWindows || os.Geteuid() != 0 {
		return nil
	}

	uidStr, ok1 := os.LookupEnv("SUDO_UID")
	gidStr, ok2 := os.LookupEnv("SUDO_GID")

	if ok1 && ok2 {
		uid, _ := strconv.Atoi(uidStr)
		gid, _ := strconv.Atoi(gidStr)

		return os.Lchown(path, uid, gid)
	}

	return nil
}

func LchownRToUser(path string) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = LchownToUser(name)
		}
		return err
	})
}

// WriteFile that does Chown when needed to drop sudo privileges.
func WriteFile(filename string, data []byte, perm fs.FileMode) error {
	err := os.WriteFile(filename, data, perm)
	if err != nil {
		return err
	}

	return ChownToUser(filename)
}

// CopyFile that does Chown when needed to drop sudo privileges.
func CopyFile(source, dest string, perm fs.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	return WriteFile(dest, data, perm)
}

// MkdirAll that does Chown when needed to drop sudo privileges.
func MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}

		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = MkdirAll(path[:j-1], perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
	}

	return err
}

// Mkdir that does Chown when needed to drop sudo privileges.
func Mkdir(path string, perm os.FileMode) error {
	err := os.Mkdir(path, perm)
	if err != nil {
		return err
	}

	return ChownToUser(path)
}

// Symlink that does Chown when needed to drop sudo privileges.
func Symlink(oldname, newname string) error {
	err := os.Symlink(oldname, newname)
	if err != nil {
		return err
	}

	return ChownToUser(newname)
}
