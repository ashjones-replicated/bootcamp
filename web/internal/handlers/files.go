package handlers

import (
	"encoding/json"
	"net/http"

	"bootcamp/web/internal/upload"
)

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	serveTemplate(w, a.Files, "templates/index.html")
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	filename, err := a.Upload.UploadFile(r.Context(), user.UploadToken, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name": filename,
		"url":  "/files/" + filename,
	})
}

func (a *App) handleListFiles(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	files, err := a.Upload.ListFiles(r.Context(), user.UploadToken, user.IsAdmin)
	if err != nil {
		a.Log.Error("list files", "err", err)
		http.Error(w, "failed to list files", http.StatusBadGateway)
		return
	}
	for i := range files {
		files[i].URL = "/files/" + files[i].Name
	}
	if files == nil {
		files = []upload.FileInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"username": user.Username,
		"is_admin": user.IsAdmin,
	})
}

func (a *App) handleFileProxy(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	filename := r.PathValue("name")
	if filename == "" {
		http.NotFound(w, r)
		return
	}
	if err := a.Upload.ProxyDownload(r.Context(), user.UploadToken, filename, w, r); err != nil {
		// Response may be partially written; log and return.
		return
	}
}

func (a *App) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	filename := r.PathValue("name")
	if filename == "" {
		http.NotFound(w, r)
		return
	}
	if err := a.Upload.DeleteFile(r.Context(), user.UploadToken, filename); err != nil {
		a.Log.Error("delete file", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
