// Package securefile provides owner-private, descriptor-anchored persistence.
package securefile

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func canonical(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("secure path must be absolute")
	}
	clean := filepath.Clean(path)
	if runtime.GOOS == "darwin" {
		if clean == "/var" || strings.HasPrefix(clean, "/var/") {
			clean = "/private" + clean
		}
		if clean == "/tmp" || strings.HasPrefix(clean, "/tmp/") {
			clean = "/private" + clean
		}
	}
	return clean, nil
}

// openDir anchors at the filesystem root and walks every component without
// following symlinks. Its returned FD remains the authority for later I/O.
func openDir(dir string, create bool) (int, error) {
	clean, err := canonical(dir)
	if err != nil {
		return -1, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/"), string(filepath.Separator))
	if len(parts) == 1 && parts[0] == "" {
		return fd, nil
	}
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			unix.Close(fd)
			return -1, errors.New("invalid secure directory component")
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		created := false
		if errors.Is(openErr, unix.ENOENT) && create {
			if mkdirErr := unix.Mkdirat(fd, part, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				unix.Close(fd)
				return -1, mkdirErr
			}
			next, openErr = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			created = true
		}
		unix.Close(fd)
		if openErr != nil {
			return -1, fmt.Errorf("open secure directory component: %w", openErr)
		}
		var stat unix.Stat_t
		if err = unix.Fstat(next, &stat); err != nil {
			unix.Close(next)
			return -1, err
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
			unix.Close(next)
			return -1, errors.New("secure path component is not a directory")
		}
		if created && (stat.Uid != uint32(unix.Geteuid()) || stat.Mode&0o077 != 0) {
			unix.Close(next)
			return -1, errors.New("created secure directory is not owner-private")
		}
		if index == len(parts)-1 && (stat.Uid != uint32(unix.Geteuid()) || stat.Mode&0o077 != 0) {
			unix.Close(next)
			return -1, errors.New("secure directory must be owned by the current user and owner-private")
		}
		fd = next
	}
	return fd, nil
}

func EnsureDir(dir string) error {
	fd, err := openDir(dir, true)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return nil
}
func validateRegular(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return err
	}
	if stat.Uid != uint32(unix.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 {
		return errors.New("secure file must be regular, current-user owned, and owner-private")
	}
	return nil
}

func Read(path string) ([]byte, error) {
	dirfd, name, err := parent(path, false)
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
	if err = validateRegular(fd); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func Write(path string, data []byte) error {
	dirfd, name, err := parent(path, true)
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)
	if existing, openErr := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0); openErr == nil {
		check := validateRegular(existing)
		unix.Close(existing)
		if check != nil {
			return errors.New("refusing to replace unsafe secure file")
		}
	} else if !errors.Is(openErr, unix.ENOENT) {
		return errors.New("refusing to replace unsafe secure file")
	}
	tmpName, tmpFD, err := createTempAt(dirfd)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(dirfd, tmpName, 0)
	tmp := os.NewFile(uintptr(tmpFD), tmpName)
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
	if err = unix.Renameat(dirfd, tmpName, dirfd, name); err != nil {
		return err
	}
	return unix.Fsync(dirfd)
}

func Remove(path string) error {
	dirfd, name, err := parent(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
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
	check := validateRegular(fd)
	unix.Close(fd)
	if check != nil {
		return errors.New("refusing to remove unsafe secure file")
	}
	if err = unix.Unlinkat(dirfd, name, 0); err != nil {
		return err
	}
	return unix.Fsync(dirfd)
}

// OpenLock returns a private regular file whose advisory lock is tied to its FD.
func OpenLock(path string) (*os.File, error) {
	dirfd, name, err := parent(path, true)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirfd)
	fd, err := unix.Openat(dirfd, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	if err = validateRegular(fd); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func parent(path string, create bool) (int, string, error) {
	clean, err := canonical(path)
	if err != nil {
		return -1, "", err
	}
	name := filepath.Base(clean)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return -1, "", errors.New("invalid secure filename")
	}
	fd, err := openDir(filepath.Dir(clean), create)
	return fd, name, err
}
func createTempAt(dirfd int) (string, int, error) {
	for i := 0; i < 128; i++ {
		var raw [12]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", -1, err
		}
		name := fmt.Sprintf(".secure-%x", raw[:])
		fd, err := unix.Openat(dirfd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
		if err == nil {
			return name, fd, nil
		}
		if !errors.Is(err, unix.EEXIST) {
			return "", -1, err
		}
	}
	return "", -1, errors.New("could not allocate secure temporary file")
}
