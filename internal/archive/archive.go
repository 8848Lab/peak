package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// TarGz archives dir into a gzip-compressed tarball written to w, always
// excluding .git/, and excluding whatever dir's own .gitignore excludes if
// one is present.
func TarGz(dir string, w io.Writer) error {
	var matcher *gitignore.GitIgnore
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
		m, err := gitignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			return err
		}
		matcher = m
	}

	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		if relSlash == ".git" || strings.HasPrefix(relSlash, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if matcher != nil && matcher.MatchesPath(relSlash) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relSlash
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})

	closeTarErr := tw.Close()
	closeGzErr := gzw.Close()

	if walkErr != nil {
		return walkErr
	}
	if closeTarErr != nil {
		return closeTarErr
	}
	return closeGzErr
}
