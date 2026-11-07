package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

const (
	scanRetentionHours = 24
	cleanupInterval    = time.Hour
	scanDir            = "/scans"
)

var htmlTemplate *template.Template

func init() {
	var err error
	htmlTemplate, err = template.ParseFiles("template.html")
	if err != nil {
		panic(fmt.Sprintf("template laden: %v", err))
	}
}

type pageData struct {
	Files []string
}

func cleanupOldScans() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	maxAge := time.Hour * time.Duration(scanRetentionHours)

	for range ticker.C {
		cutoff := time.Now().Add(-maxAge)

		entries, err := os.ReadDir(scanDir)
		if err != nil {
			fmt.Printf("Fehler beim Lesen von %s: %v\n", scanDir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if len(name) < 5 || name[:5] != "scan-" {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				path := filepath.Join(scanDir, name)
				if err := os.Remove(path); err != nil {
					fmt.Printf("Konnte %s nicht löschen: %v\n", name, err)
				} else {
					fmt.Printf("Alter Scan gelöscht: %s\n", name)
				}
			}
		}
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		http.Error(w, "Fehler beim Lesen des Scan-Verzeichnisses", http.StatusInternalServerError)
		return
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	data := pageData{Files: files}
	if err := htmlTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Methode nicht erlaubt", http.StatusMethodNotAllowed)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	output := filepath.Join(scanDir, fmt.Sprintf("scan-%s.png", timestamp))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "scanimage", "--format=png", "--resolution", "300")

	outFile, err := os.Create(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("Datei konnte nicht erstellt werden: %v", err), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		outFile.Close()
		os.Remove(output)

		if ctx.Err() == context.DeadlineExceeded {
			http.Error(w, "Scanner hat nicht innerhalb des Zeitlimits reagiert.", http.StatusInternalServerError)
			return
		}

		http.Error(w, fmt.Sprintf("Scan fehlgeschlagen:<pre>%s</pre>", stderr.String()), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filepath.Base(filename) != filename {
		http.Error(w, "Ungültiger Dateiname", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(scanDir, filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, filePath)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Methode nicht erlaubt", http.StatusMethodNotAllowed)
		return
	}

	filename := r.PathValue("filename")
	if filepath.Base(filename) != filename {
		http.Error(w, "Ungültiger Dateiname", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(scanDir, filename)

	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		http.Error(w, "Datei nicht gefunden", http.StatusNotFound)
		return
	}

	if err := os.Remove(filePath); err != nil {
		http.Error(w, fmt.Sprintf("Datei konnte nicht gelöscht werden: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func main() {
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		panic(fmt.Sprintf("scan-dir erstellen: %v", err))
	}

	go cleanupOldScans()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", indexHandler)
	mux.HandleFunc("POST /scan", scanHandler)
	mux.HandleFunc("GET /download/{filename}", downloadHandler)
	mux.HandleFunc("POST /delete/{filename}", deleteHandler)

	fmt.Println("Server startet auf :8080")
	if err := http.ListenAndServe("0.0.0.0:8080", mux); err != nil {
		panic(fmt.Sprintf("Server: %v", err))
	}
}
