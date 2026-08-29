package securefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
