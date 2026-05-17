package oaklog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const DefaultPastebinEndpoint = "https://pastebin.com/api/api_post.php"
const maxPastebinResponseBody = 1 << 20

type PastebinClient struct {
	Endpoint   string
	HTTPClient *http.Client
	APIKey     string
	Private    string
	ExpireDate string
	Format     string
}

func (c *PastebinClient) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	apiKey := strings.TrimSpace(c.APIKey)
	if apiKey == "" {
		return UploadResult{}, errors.New(pastebinAPIKeyError)
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultPastebinEndpoint
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{}
	form.Set("api_dev_key", apiKey)
	form.Set("api_option", "paste")
	form.Set("api_paste_code", string(req.Content))
	form.Set("api_paste_name", req.Source)
	form.Set("api_paste_private", defaultPastebinValue(c.Private, pastebinVisibilityPublic))
	form.Set("api_paste_expire_date", defaultPastebinValue(c.ExpireDate, "1W"))
	form.Set("api_paste_format", defaultPastebinValue(c.Format, "text"))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return UploadResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "oaklog/"+Version+" (+https://github.com/rannday/oaklog)")

	resp, err := client.Do(httpReq)
	if err != nil {
		return UploadResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPastebinResponseBody))
	if err != nil {
		return UploadResult{}, err
	}

	text := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if strings.HasPrefix(text, "Bad API request") {
			return UploadResult{}, fmt.Errorf("pastebin upload failed: %s", text)
		}
		if text != "" {
			return UploadResult{}, fmt.Errorf("pastebin upload failed: %s: %s", resp.Status, text)
		}
		return UploadResult{}, fmt.Errorf("pastebin upload failed: %s", resp.Status)
	}

	if text == "" {
		return UploadResult{}, errors.New("pastebin upload failed: empty response")
	}
	if strings.HasPrefix(text, "Bad API request") {
		return UploadResult{}, fmt.Errorf("pastebin upload failed: %s", text)
	}

	parsed, err := url.Parse(text)
	if err != nil {
		return UploadResult{}, fmt.Errorf("pastebin upload failed: invalid response URL")
	}
	if parsed.Scheme != "https" || parsed.Host != "pastebin.com" {
		return UploadResult{}, fmt.Errorf("pastebin upload failed: invalid response URL")
	}

	key := pastebinKeyFromPath(parsed.Path)
	if key == "" {
		return UploadResult{}, fmt.Errorf("pastebin upload failed: invalid response URL")
	}

	return UploadResult{
		Provider: string(ProviderPastebin),
		ID:       key,
		URL:      text,
		Raw:      "https://pastebin.com/raw/" + key,
		Size:     int64(len(req.Content)),
		Lines:    int64(countLogLines(req.Content)),
	}, nil
}

func defaultPastebinValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func pastebinKeyFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
