package sponsorblock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// HTTPClient is the key-free SponsorBlock implementation. It reads the public
// skip-segments endpoint, asking for the sponsor category only.
type HTTPClient struct {
	http    *http.Client
	baseURL string
}

var _ Client = (*HTTPClient)(nil)

const defaultBaseURL = "https://sponsor.ajay.app"

// NewClient wraps httpClient (nil means http.DefaultClient).
func NewClient(httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &HTTPClient{http: httpClient, baseURL: defaultBaseURL}
}

// skipSegment is the slice of a SponsorBlock skip-segments entry the CLI reads.
type skipSegment struct {
	Segment  []float64 `json:"segment"`
	UUID     string    `json:"UUID"`
	Category string    `json:"category"`
}

// FetchSlots asks SponsorBlock for the Video's sponsor-category segments.
func (c *HTTPClient) FetchSlots(ctx context.Context, videoID string) ([]Window, error) {
	q := url.Values{"videoID": {videoID}, "category": {"sponsor"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/skipSegments?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// SponsorBlock answers 404 for a Video nobody has submitted segments
		// for; that is "no data", not a failure.
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: %s: HTTP %d", videoID, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: %s: %w", videoID, err)
	}

	var segments []skipSegment
	if err := json.Unmarshal(body, &segments); err != nil {
		return nil, fmt.Errorf("sponsorblock: %s: decode response: %w", videoID, err)
	}
	windows := make([]Window, 0, len(segments))
	for _, s := range segments {
		// Only the sponsor category is requested; filter anyway so a change
		// upstream cannot turn self-promotion into Slots.
		if s.Category != "sponsor" {
			continue
		}
		if len(s.Segment) != 2 || s.Segment[1] < s.Segment[0] {
			return nil, fmt.Errorf("sponsorblock: %s: segment %s has bounds %v", videoID, s.UUID, s.Segment)
		}
		windows = append(windows, Window{Start: s.Segment[0], End: s.Segment[1], ID: s.UUID})
	}
	return windows, nil
}
