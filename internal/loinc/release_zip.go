package loinc

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractReleaseZip(zipPath string, targetDir string) (string, error) {
	if err := unzip(zipPath, targetDir); err != nil {
		return "", err
	}
	return FindReleaseDir(targetDir)
}

func FindReleaseDir(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		candidate := filepath.Join(path, "LoincTable", "Loinc.csv")
		if _, err := os.Stat(candidate); err == nil {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("release zip does not contain LoincTable/Loinc.csv")
	}
	return found, nil
}

func unzip(zipPath string, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open release zip: %w", err)
	}
	defer reader.Close()

	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(filepath.Separator)) || name == ".." {
			return fmt.Errorf("unsafe zip path %q", file.Name)
		}
		dest := filepath.Join(targetDir, name)
		destAbs, err := filepath.Abs(dest)
		if err != nil {
			return err
		}
		if destAbs != targetAbs && !strings.HasPrefix(destAbs, targetAbs+string(filepath.Separator)) {
			return fmt.Errorf("unsafe zip path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("create zip directory: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create zip parent directory: %w", err)
		}
		in, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return fmt.Errorf("create zip entry: %w", err)
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			return fmt.Errorf("extract zip entry: %w", err)
		}
		in.Close()
		if err := out.Close(); err != nil {
			return fmt.Errorf("close zip entry: %w", err)
		}
	}
	return nil
}
