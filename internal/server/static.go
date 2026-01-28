package server

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// MIMETypes maps file extensions to content types.
var MIMETypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
	".json": "application/json; charset=utf-8",
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".ico":  "image/x-icon",
}

// ServeStatic serves a static file from siteDir.
func ServeStatic(w http.ResponseWriter, pathname string, siteDir string) {
	cleanPath := pathname
	if cleanPath == "/" {
		cleanPath = "/index.html"
	}

	safePath := path.Clean(cleanPath)
	safePath = strings.TrimPrefix(safePath, "/")

	filePath := filepath.Join(siteDir, safePath)

	// Prevent path traversal
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	absSite, err := filepath.Abs(siteDir)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	if !strings.HasPrefix(absFile, absSite) {
		http.Error(w, "Forbidden", 403)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	if info.IsDir() {
		http.Error(w, "Forbidden", 403)
		return
	}

	ext := filepath.Ext(filePath)
	contentType, ok := MIMETypes[ext]
	if !ok {
		contentType = "application/octet-stream"
	}

	cacheControl := "public, max-age=300"
	if ext == ".json" {
		cacheControl = "no-store"
	}

	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	io.Copy(w, f)
}
