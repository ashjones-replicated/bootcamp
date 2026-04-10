package replicated

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Update represents an available application update returned by the Replicated SDK.
type Update struct {
	VersionLabel string `json:"versionLabel"`
}

// UpdatesClient checks for available updates via the Replicated in-cluster SDK API.
// Results are cached for five minutes to reflect new releases promptly.
type UpdatesClient struct {
	sdkURL   string
	client   *http.Client
	mu       sync.Mutex
	cached   []Update
	cachedAt time.Time
}

func NewUpdatesClient(sdkURL string) *UpdatesClient {
	return &UpdatesClient{
		sdkURL: sdkURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckUpdates returns the list of available updates from the SDK.
// Errors are returned so callers can decide whether to surface or ignore them.
func (c *UpdatesClient) CheckUpdates(ctx context.Context) ([]Update, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cachedAt.IsZero() && time.Since(c.cachedAt) < 5*time.Minute {
		return c.cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sdkURL+"/api/v1/app/updates", nil)
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

	var updates []Update
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	c.cached = updates
	c.cachedAt = time.Now()
	return updates, nil
}
