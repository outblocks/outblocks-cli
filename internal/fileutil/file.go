package fileutil

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/pkg/errors"
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

// CopyFileContents that copies file contents and does Chown when needed to drop sudo privileges.
func CopyFileContents(source, dest string, perm fs.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	return WriteFile(dest, data, perm)
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist.
func CopyDir(src, dst string, filter func(path string, entry fs.DirEntry) bool) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	fi, err := os.Lstat(src)
	if err != nil {
		return merry.Wrap(err)
	}

	if !fi.IsDir() {
		return merry.New("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(dst, fi.Mode()); err != nil {
				return merry.Errorf("cannot mkdir %s: %w", dst, err)
			}
		} else {
			return merry.Wrap(err)
		}
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return merry.Errorf("cannot read directory %s: %w", dst, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if filter != nil && !filter(srcPath, entry) {
			continue
		}

		if entry.IsDir() {
			if err = CopyDir(srcPath, dstPath, filter); err != nil {
				return merry.Errorf("copying directory failed: %w", err)
			}
		} else {
			// This will include symlinks, which is what we want when
			// copying things.
			if err = copyFile(srcPath, dstPath); err != nil {
				return merry.Errorf("copying file failed: %w", err)
			}
		}
	}

	return nil
}

// copyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all its contents will be replaced by the contents
// of the source file. The file mode will be copied from the source.
func copyFile(src, dst string) error {
	sym, err := IsSymlink(src)
	if err != nil {
		return errors.Wrap(err, "symlink check failed")
	}

	if sym {
		err := cloneSymlink(src, dst)
		if err != nil && runtime.GOOS == "windows" {
			// If cloning the symlink fails on Windows because the user
			// does not have the required privileges, ignore the error and
			// fall back to copying the file contents.
			//
			// ERROR_PRIVILEGE_NOT_HELD is 1314 (0x522):
			// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681385(v=vs.85).aspx
			if lerr, ok := err.(*os.LinkError); ok && lerr.Err != syscall.Errno(1314) {
				return err
			}
		}

		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	// Check for write errors on Close
	if err := out.Close(); err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, si.Mode())

	return err
}

// IsSymlink determines if the given path is a symbolic link.
func IsSymlink(path string) (bool, error) {
	l, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	return l.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

// cloneSymlink will create a new symlink that points to the resolved path of sl.
// If sl is a relative symlink, dst will also be a relative symlink.
func cloneSymlink(sl, dst string) error {
	resolved, err := os.Readlink(sl)
	if err != nil {
		return err
	}

	return os.Symlink(resolved, dst)
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

func GetModTimeFromFile(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}

	return fi.ModTime()
}

func Touch(path string) error {
	n := time.Now()

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	_ = f.Close()

	return os.Chtimes(path, n, n)
}
