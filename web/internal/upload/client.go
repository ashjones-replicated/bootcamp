package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Client struct {
	baseURL    string
	adminToken string
	httpClient *http.Client
}

type FileInfo struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	UploadedAt int64  `json:"uploaded_at"`
	UploadedBy string `json:"uploaded_by,omitempty"`
}

func New(baseURL, adminToken string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		adminToken: adminToken,
		httpClient: &http.Client{},
	}
}

// UploadFile proxies an upload from an incoming request to the upload service.
// Returns the bare filename (e.g. "ab12.png") on success.
func (c *Client) UploadFile(ctx context.Context, token string, r *http.Request) (string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", fmt.Errorf("parse form: %w", err)
	}

	f, header, err := r.FormFile("file")
	if err != nil {
		return "", fmt.Errorf("get file field: %w", err)
	}
	defer f.Close()

	private := r.FormValue("private") == "on"
	single := r.FormValue("single") == "on"

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", header.Filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	if private {
		if err := mw.WriteField("private", "on"); err != nil {
			return "", err
		}
	}
	if single {
		if err := mw.WriteField("single", "on"); err != nil {
			return "", err
		}
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authentication", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload service %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	rawURL := strings.TrimSpace(string(body))
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse upload response URL: %w", err)
	}
	return path.Base(parsed.Path), nil
}

// ListFiles returns the files visible to the given token.
// When all is true the request includes ?all, causing the upload service to
// return every user's files (only effective when the token belongs to an admin).
func (c *Client) ListFiles(ctx context.Context, token string, all bool) ([]FileInfo, error) {
	endpoint := c.baseURL + "/files?json"
	if all {
		endpoint += "&all"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authentication", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list files: status %d", resp.StatusCode)
	}

	var raw []struct {
		FileName string `json:"fileName"`
		Ttl      int64  `json:"ttl"`
		User     string `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("list files: decode: %w", err)
	}
	files := make([]FileInfo, len(raw))
	for i, f := range raw {
		files[i] = FileInfo{
			Name:       f.FileName,
			UploadedAt: f.Ttl,
			UploadedBy: f.User,
		}
	}
	return files, nil
}

// CountAllFiles returns the total number of files across all users (requires admin token).
func (c *Client) CountAllFiles(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/files?json&all", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authentication", c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("count files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("count files: status %d", resp.StatusCode)
	}

	var files []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return 0, fmt.Errorf("count files: decode: %w", err)
	}
	return len(files), nil
}

// ProxyDownload forwards a file download from the upload service to the client.
func (c *Client) ProxyDownload(ctx context.Context, token, filename string, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+filename, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authentication", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

// DeleteFile removes a file from the upload service.
func (c *Client) DeleteFile(ctx context.Context, token, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/"+filename, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authentication", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete file: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// CreateToken provisions a new token in the upload service and returns the raw token string.
func (c *Client) CreateToken(ctx context.Context, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/users/"+url.PathEscape(name), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authentication", c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create token: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return strings.TrimSpace(string(body)), nil
}

// DeleteToken revokes a token in the upload service.
func (c *Client) DeleteToken(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.baseURL+"/users/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authentication", c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete token: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
