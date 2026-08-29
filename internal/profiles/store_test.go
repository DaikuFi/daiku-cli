package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndRejectsTraversalNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	s := Store{Path: path}
	cfg := Config{Current: "work", Profiles: map[string]Profile{"work": {APIURL: "https://api.daiku.app/api/v1/"}}}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || got.Current != "work" {
		t.Fatalf("got %#v, %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	for _, name := range []string{"../x", "a/b", "", ".."} {
		if ValidateName(name) == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}
func TestStoreRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if err := (Store{Path: link}).Save(Config{}); err == nil {
		t.Fatal("expected symlink denial")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "keep" {
		t.Fatal("target changed")
	}
}
