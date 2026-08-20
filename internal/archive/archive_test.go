package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func entryNames(t *testing.T, data []byte) []string {
	t.Helper()
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		names = append(names, header.Name)
	}
	return names
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

type tarEntry struct {
	Name     string
	Typeflag byte
	Linkname string
}

func entries(t *testing.T, data []byte) []tarEntry {
	t.Helper()
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	var out []tarEntry
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		out = append(out, tarEntry{Name: header.Name, Typeflag: header.Typeflag, Linkname: header.Linkname})
	}
	return out
}

func TestTarGzIncludesRegularFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name": "demo"}`)
	writeFile(t, filepath.Join(dir, "src", "index.js"), "console.log(1)")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "package.json") {
		t.Errorf("missing package.json in %v", names)
	}
	if !contains(names, "src/index.js") {
		t.Errorf("missing src/index.js in %v", names)
	}
}

func TestTarGzAlwaysExcludesGitDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	for _, n := range names {
		if n == ".git/HEAD" || n == ".git/" {
			t.Errorf(".git contents leaked into archive: %v", names)
		}
	}
}

func TestTarGzExcludesGitignoredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	writeFile(t, filepath.Join(dir, "node_modules", "leftpad", "index.js"), "module.exports = {}")
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "package.json") {
		t.Errorf("missing package.json in %v", names)
	}
	if !contains(names, ".gitignore") {
		t.Errorf("missing .gitignore itself in %v", names)
	}
	for _, n := range names {
		if n == "node_modules/leftpad/index.js" {
			t.Errorf("node_modules leaked into archive despite .gitignore: %v", names)
		}
	}
}

func TestTarGzHandlesSymlinksWithoutFollowingContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello world")

	linkPath := filepath.Join(dir, "link.txt")
	if err := os.Symlink("target.txt", linkPath); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "privilege") ||
			strings.Contains(strings.ToLower(err.Error()), "permission") ||
			errors.Is(err, os.ErrPermission) {
			t.Skipf("skipping: creating symlinks requires elevated privileges here: %v", err)
		}
		t.Fatalf("os.Symlink: %v", err)
	}

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	found := false
	for _, e := range entries(t, buf.Bytes()) {
		if e.Name == "link.txt" {
			found = true
			if e.Typeflag != tar.TypeSymlink {
				t.Errorf("expected link.txt to be TypeSymlink, got %v", e.Typeflag)
			}
			if e.Linkname != "target.txt" {
				t.Errorf("expected Linkname %q, got %q", "target.txt", e.Linkname)
			}
		}
	}
	if !found {
		t.Errorf("link.txt entry not found in archive")
	}
}

func TestTarGzWithNoGitignoreIncludesEverythingExceptGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	writeFile(t, filepath.Join(dir, "node_modules", "x.js"), "x")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "node_modules/x.js") {
		t.Errorf("expected node_modules/x.js to be included with no .gitignore present: %v", names)
	}
}
