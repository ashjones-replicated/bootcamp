package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func (a *App) handleGenerateSupportBundle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	outPath := fmt.Sprintf("/tmp/support-bundle-%d.tar.gz", time.Now().UnixNano())
	defer os.Remove(outPath)

	cmd := exec.CommandContext(ctx, "/support-bundle",
		"--load-cluster-specs",
		"--namespace", a.Cfg.PodNamespace,
		"--interactive=false",
		"--output", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		a.Log.Error("support bundle generation failed", "err", err, "output", string(out))
		http.Error(w, "failed to generate support bundle", http.StatusInternalServerError)
		return
	}

	f, err := os.Open(outPath)
	if err != nil {
		a.Log.Error("open support bundle", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		a.Log.Error("stat support bundle", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	uploadURL := fmt.Sprintf("%s/api/v1/supportbundle", a.Cfg.ReplicatedSDKURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, f)
	if err != nil {
		a.Log.Error("create upload request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/gzip")
	req.ContentLength = fi.Size()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.Log.Error("upload support bundle", "err", err)
		http.Error(w, "failed to upload support bundle", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		a.Log.Error("upload failed", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "failed to upload support bundle", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(body)
}
