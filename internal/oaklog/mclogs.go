package oaklog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultMCLogsEndpoint = "https://api.mclo.gs/1/log"
const maxMCLogsResponseBody = 1 << 20

type MCLogsClient struct {
	Endpoint   string
	HTTPClient *http.Client
}

type mclogsUploadRequest struct {
	Content string `json:"content"`
	Source  string `json:"source"`
}

type mclogsUploadResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	Source  any    `json:"source"`
	Created int64  `json:"created"`
	Expires int64  `json:"expires"`
	Size    int64  `json:"size"`
	Lines   int64  `json:"lines"`
	Errors  int64  `json:"errors"`
	URL     string `json:"url"`
	Raw     string `json:"raw"`
	Token   string `json:"token"` // parsed but intentionally not exposed; it can authorize later log actions
	Error   string `json:"error"`
}

func (c *MCLogsClient) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultMCLogsEndpoint
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	body, err := json.Marshal(mclogsUploadRequest{
		Content: string(req.Content),
		Source:  req.Source,
	})
	if err != nil {
		return UploadResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return UploadResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "oaklog/"+Version+" (+https://github.com/rannday/oaklog)")

	resp, err := client.Do(httpReq)
	if err != nil {
		return UploadResult{}, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxMCLogsResponseBody))
	if err != nil {
		return UploadResult{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var decoded mclogsUploadResponse
		if json.Unmarshal(body, &decoded) == nil && decoded.Error != "" {
			return UploadResult{}, fmt.Errorf("mclo.gs upload failed: %s: %s", resp.Status, decoded.Error)
		}
		if len(body) > 0 {
			return UploadResult{}, fmt.Errorf("mclo.gs upload failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return UploadResult{}, fmt.Errorf("mclo.gs upload failed: %s", resp.Status)
	}

	var decoded mclogsUploadResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return UploadResult{}, err
	}

	if !decoded.Success {
		if decoded.Error == "" {
			return UploadResult{}, errors.New("mclo.gs upload failed")
		}
		return UploadResult{}, errors.New(decoded.Error)
	}
	if decoded.URL == "" {
		return UploadResult{}, errors.New("mclo.gs upload succeeded but returned an empty url")
	}

	id := decoded.ID
	if id == "" {
		id = strings.TrimPrefix(decoded.URL, "https://mclo.gs/")
	}

	return UploadResult{
		Provider: string(ProviderMCLogs),
		ID:       id,
		URL:      decoded.URL,
		Raw:      decoded.Raw,
		Size:     decoded.Size,
		Lines:    decoded.Lines,
		Errors:   decoded.Errors,
		Expires:  decoded.Expires,
	}, nil
}
