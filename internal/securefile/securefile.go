// Package securefile provides owner-private, symlink-resistant persistence for
// the macOS and Linux targets supported by daiku-cli.
package securefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func EnsureDir(dir string) error {
	dir = filepath.Clean(dir)
	ancestor := dir
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return err
		}
		ancestor = next
	}
	ancestorInfo, err := os.Lstat(ancestor)
	if err != nil || ancestorInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("secure directory parent is a symbolic link")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("secure directory must be owner-private and must not be a symbolic link")
	}
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open secure directory: %w", err)
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return err
	}
	if stat.Uid != uint32(unix.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0o077 != 0 {
		return errors.New("secure directory must be owned by the current user and owner-private")
	}
	return nil
}

func Read(path string) ([]byte, error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if err := EnsureDir(dir); err != nil {
		return nil, err
	}
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirfd)
	fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Uid != uint32(unix.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 {
		return nil, errors.New("secure file must be regular and owner-private")
	}
	return io.ReadAll(file)
}

func Write(path string, data []byte) error {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		fd, openErr := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil {
			return errors.New("refusing to replace unsafe secure file")
		}
		var stat unix.Stat_t
		statErr := unix.Fstat(fd, &stat)
		unix.Close(fd)
		if statErr != nil || stat.Uid != uint32(unix.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 {
			return errors.New("refusing to replace unsafe secure file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".secure-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)
	if err = unix.Renameat(unix.AT_FDCWD, tmpName, dirfd, name); err != nil {
		return err
	}
	return unix.Fsync(dirfd)
}

func Remove(path string) error {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)
	fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return errors.New("refusing to remove unsafe secure file")
	}
	var stat unix.Stat_t
	statErr := unix.Fstat(fd, &stat)
	unix.Close(fd)
	if statErr != nil || stat.Uid != uint32(unix.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 {
		return errors.New("refusing to remove unsafe secure file")
	}
	if err = unix.Unlinkat(dirfd, name, 0); err != nil {
		return err
	}
	return unix.Fsync(dirfd)
}
