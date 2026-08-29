package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTripPermissionsAndDelete(t *testing.T) {
	s := FileStore{Dir: filepath.Join(t.TempDir(), "credentials")}
	want := Token{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: 42}
	if err := s.Put("work", want); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(s.path("work"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	got, err := s.Get("work")
	if err != nil || got != want {
		t.Fatalf("got %#v %v", got, err)
	}
	if err = s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get("work"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
func TestFileStoreRejectsSymlinkAndLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "credentials")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := FileStore{Dir: dir}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, s.path("work")); err != nil {
		t.Skip(err)
	}
	if err := s.Put("work", Token{}); err == nil {
		t.Fatal("expected symlink denial")
	}
	os.Remove(s.path("work"))
	if err := os.WriteFile(s.path("work"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("work"); err == nil {
		t.Fatal("expected permission denial")
	}
}
