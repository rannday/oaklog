package oaklog

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestPastebinUploadSuccess(t *testing.T) {
	var gotMethod string
	var gotContentType string
	var gotUserAgent string
	var gotForm string
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotForm = r.PostForm.Encode()
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("https://pastebin.com/UIFdu235s"))
	}))

	c := PastebinClient{
		Endpoint:   "http://example.invalid/api_post.php",
		HTTPClient: client,
		APIKey:     "dev-key",
	}
	result, err := c.Upload(context.Background(), UploadRequest{Content: []byte("line 1\nline 2\n"), Source: "oaklog"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("expected form content-type, got %q", gotContentType)
	}
	if !strings.HasPrefix(gotUserAgent, "oaklog/") {
		t.Fatalf("unexpected user-agent: %q", gotUserAgent)
	}
	if !strings.Contains(gotForm, "api_option=paste") || !strings.Contains(gotForm, "api_dev_key=dev-key") || !strings.Contains(gotForm, "api_paste_code=line+1%0Aline+2%0A") {
		t.Fatalf("unexpected form body: %s", gotForm)
	}
	if !strings.Contains(gotForm, "api_paste_name=oaklog") {
		t.Fatalf("expected paste name in form body: %s", gotForm)
	}
	if !strings.Contains(gotForm, "api_paste_private=0") || !strings.Contains(gotForm, "api_paste_expire_date=1W") || !strings.Contains(gotForm, "api_paste_format=text") {
		t.Fatalf("expected default paste settings in form body: %s", gotForm)
	}
	if result.Provider != string(ProviderPastebin) || result.ID != "UIFdu235s" || result.URL != "https://pastebin.com/UIFdu235s" || result.Raw != "https://pastebin.com/raw/UIFdu235s" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Size != int64(len([]byte("line 1\nline 2\n"))) {
		t.Fatalf("unexpected size: %+v", result)
	}
	if result.Lines != 2 {
		t.Fatalf("unexpected lines: %+v", result)
	}
}

func TestPastebinUploadAPIError(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad API request, invalid api_dev_key"))
	}))

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client, APIKey: "dev-key"}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "Bad API request") {
		t.Fatalf("expected api error, got %v", err)
	}
}

func TestPastebinUploadNon2xx(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client, APIKey: "dev-key"}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "403 Forbidden") || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected non-2xx error, got %v", err)
	}
}

func TestPastebinUploadMissingAPIKey(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent")
	}))

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "PASTEBIN_API is required when using --pastebin") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestPastebinUploadMalformedSuccessBody(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a url"))
	}))

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client, APIKey: "dev-key"}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "invalid response URL") {
		t.Fatalf("expected malformed success error, got %v", err)
	}
}

func TestPastebinUploadEmptyBody(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client, APIKey: "dev-key"}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("expected empty body error, got %v", err)
	}
}

func TestPastebinUploadPublicAndUnlisted(t *testing.T) {
	tests := []struct {
		name     string
		private  string
		expected string
	}{
		{name: "public", private: pastebinVisibilityPublic, expected: "api_paste_private=0"},
		{name: "unlisted", private: pastebinVisibilityUnlisted, expected: "api_paste_private=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotForm string
			client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm: %v", err)
				}
				gotForm = r.PostForm.Encode()
				_, _ = w.Write([]byte("https://pastebin.com/UIFdu235s"))
			}))
			c := PastebinClient{
				Endpoint:   "http://example.invalid/api_post.php",
				HTTPClient: client,
				APIKey:     "dev-key",
				Private:    tt.private,
			}
			if _, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"}); err != nil {
				t.Fatalf("Upload returned error: %v", err)
			}
			if !strings.Contains(gotForm, tt.expected) {
				t.Fatalf("expected %s in form body: %s", tt.expected, gotForm)
			}
		})
	}
}

func TestPastebinUploadCapsResponseBodyReads(t *testing.T) {
	body := &trackingBody{limit: maxPastebinResponseBody + 4096}
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Header:     make(http.Header),
				Body:       body,
				Request:    req,
			}, nil
		}),
	}

	c := PastebinClient{Endpoint: "http://example.invalid/api_post.php", HTTPClient: client, APIKey: "dev-key"}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil {
		t.Fatal("expected upload error")
	}
	if body.read > maxPastebinResponseBody {
		t.Fatalf("expected capped read, got %d bytes", body.read)
	}
}
