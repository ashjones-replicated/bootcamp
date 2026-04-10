package replicated

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// licenseField mirrors the per-field object returned by the SDK's
// GET /api/v1/license/fields endpoint.
// Value is any because the SDK encodes it as the native JSON type
// (boolean, number, or string) rather than always quoting it as a string.
type licenseField struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Value     any    `json:"value"`
	ValueType string `json:"valueType"`
}

// licenseInfo mirrors the response from GET /api/v1/license/info.
// Expiry lives in entitlements["expires_at"].Value, not as a top-level field.
type licenseInfo struct {
	Entitlements map[string]licenseField `json:"entitlements"`
}

// LicenseClient reads license information from the Replicated
// in-cluster SDK API. Results are cached for five minutes.
type LicenseClient struct {
	sdkURL       string
	client       *http.Client
	mu           sync.Mutex
	cached       map[string]licenseField
	cachedAt     time.Time
	cachedInfo   *licenseInfo
	cachedInfoAt time.Time
}

func NewLicenseClient(sdkURL string) *LicenseClient {
	return &LicenseClient{
		sdkURL: sdkURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *LicenseClient) fields(ctx context.Context) (map[string]licenseField, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cachedAt.IsZero() && time.Since(c.cachedAt) < 5*time.Minute {
		return c.cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sdkURL+"/api/v1/license/fields", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sdk request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sdk response: status %d", resp.StatusCode)
	}

	var fields map[string]licenseField
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	c.cached = fields
	c.cachedAt = time.Now()
	return fields, nil
}

func (c *LicenseClient) info(ctx context.Context) (*licenseInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedInfo != nil && time.Since(c.cachedInfoAt) < 5*time.Minute {
		return c.cachedInfo, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sdkURL+"/api/v1/license/info", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sdk request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sdk response: status %d", resp.StatusCode)
	}

	var info licenseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	c.cachedInfo = &info
	c.cachedInfoAt = time.Now()
	return &info, nil
}

// IsExpired reports whether the license has passed its expiry date.
// Defaults to false when the SDK is unreachable or no expiry is set.
func (c *LicenseClient) IsExpired(ctx context.Context) (bool, error) {
	info, err := c.info(ctx)
	if err != nil {
		return false, err
	}
	f, ok := info.Entitlements["expires_at"]
	if !ok {
		return false, nil
	}
	v, _ := f.Value.(string)
	if v == "" {
		return false, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return false, fmt.Errorf("parse expires_at: %w", err)
	}
	return time.Now().After(t), nil
}

// AllowUserCreation reports whether the license permits creating new users.
// Defaults to true when the field is absent or the SDK is unreachable, so
// that development environments (no SDK) are unaffected.
func (c *LicenseClient) AllowUserCreation(ctx context.Context) (bool, error) {
	fields, err := c.fields(ctx)
	if err != nil {
		return true, err
	}
	f, ok := fields["allow_user_creation"]
	if !ok {
		return true, nil
	}
	switch v := f.Value.(type) {
	case bool:
		return v, nil
	case string:
		return v == "true", nil
	default:
		return fmt.Sprintf("%v", f.Value) == "true", nil
	}
}
