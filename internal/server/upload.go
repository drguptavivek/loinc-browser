package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loinc-browser/internal/loinc"
)

const maxUploadSize = 512 << 20

type uploadResponse struct {
	OK         bool   `json:"ok"`
	TermCount  int    `json:"termCount"`
	DBPath     string `json:"dbPath"`
	ReleaseDir string `json:"releaseDir"`
	ImportedAt string `json:"importedAt"`
}

func (a *app) uploadImport(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(a.dbPath) == "" {
		writeError(w, http.StatusBadRequest, errors.New("server DB path is not configured"))
		return
	}
	uploadDir := a.uploadDir
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = "./data/uploads"
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse upload: %w", err))
		return
	}
	file, header, err := r.FormFile("releaseZip")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("multipart field releaseZip is required"))
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		writeError(w, http.StatusBadRequest, errors.New("uploaded release must be a .zip file"))
		return
	}

	workDir := filepath.Join(uploadDir, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create upload directory: %w", err))
		return
	}
	zipPath := filepath.Join(workDir, safeUploadName(header.Filename))
	out, err := os.Create(zipPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create uploaded zip: %w", err))
		return
	}
	if _, err := io.Copy(out, io.LimitReader(file, maxUploadSize+1)); err != nil {
		out.Close()
		writeError(w, http.StatusInternalServerError, fmt.Errorf("save uploaded zip: %w", err))
		return
	}
	if err := out.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("close uploaded zip: %w", err))
		return
	}

	extractDir := filepath.Join(workDir, "release")
	releaseDir, err := loinc.ExtractReleaseZip(zipPath, extractDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tempDB := a.dbPath + ".uploading"
	removeSQLiteFiles(tempDB)
	summary, err := loinc.Ingest(context.Background(), loinc.IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     tempDB,
	})
	if err != nil {
		removeSQLiteFiles(tempDB)
		writeError(w, http.StatusBadRequest, fmt.Errorf("ingest uploaded release: %w", err))
		return
	}

	if err := swapDatabaseFile(tempDB, a.dbPath); err != nil {
		removeSQLiteFiles(tempDB)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	store, err := loinc.OpenStore(a.dbPath, loinc.StoreOptions{CacheEntries: a.cacheEntries})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("open uploaded database: %w", err))
		return
	}
	if err := a.swapStore(store); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("swap uploaded database: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, uploadResponse{
		OK:         true,
		TermCount:  summary.TermCount,
		DBPath:     a.dbPath,
		ReleaseDir: releaseDir,
		ImportedAt: summary.ImportedAt.Format(time.RFC3339),
	})
}

func swapDatabaseFile(tempDB string, dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	removeSQLiteFiles(dbPath)
	if err := os.Rename(tempDB, dbPath); err != nil {
		return fmt.Errorf("replace database: %w", err)
	}
	return nil
}

func removeSQLiteFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

func safeUploadName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	if name == "." || name == "" {
		return "loinc-release.zip"
	}
	return name
}
