package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
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
