package securefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRejectsSymlinkParentAndLooseDirectory(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	if err := Write(filepath.Join(link, "secret"), []byte("x")); err == nil {
		t.Fatal("accepted symlink parent")
	}
	loose := filepath.Join(root, "loose")
	if err := os.Mkdir(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(loose, "secret"), []byte("x")); err == nil {
		t.Fatal("accepted loose directory")
	}
}

func TestReadAndRemoveRejectReplacementSymlink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secure")
	path := filepath.Join(dir, "secret")
	if err := Write(path, []byte("credential")); err != nil {
		t.Fatal(err)
	}
	if err := Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(dir), "target")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skip(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("read followed replacement symlink")
	}
	if err := Remove(path); err == nil {
		t.Fatal("remove followed replacement symlink")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "keep" {
		t.Fatal("symlink target changed")
	}
}

func TestMissingFilePreservesNotExist(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "secure", "missing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestRejectsExistingChildBehindIntermediateSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	child := filepath.Join(real, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	if err := Write(filepath.Join(link, "child", "secret"), []byte("x")); err == nil {
		t.Fatal("walk followed intermediate symlink")
	}
}

func TestAnchoredDirectorySurvivesAncestorSwap(t *testing.T) {
	root := t.TempDir()
	ancestor := filepath.Join(root, "ancestor")
	secure := filepath.Join(ancestor, "secure")
	if err := os.MkdirAll(secure, 0o700); err != nil {
		t.Fatal(err)
	}
	fd, err := openDir(secure, false)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	moved := filepath.Join(root, "moved")
	if err = os.Rename(ancestor, moved); err != nil {
		t.Fatal(err)
	}
	attacker := filepath.Join(root, "attacker")
	if err = os.Mkdir(attacker, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(attacker, ancestor); err != nil {
		t.Skip(err)
	}
	secretFD, err := unix.Openat(fd, "secret", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	unix.Close(secretFD)
	if _, err = os.Stat(filepath.Join(moved, "secure", "secret")); err != nil {
		t.Fatal("anchored target missing")
	}
	if _, err = os.Stat(filepath.Join(attacker, "secret")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("write escaped to swapped ancestor")
	}
}
