package oaklog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCLogsUploadSuccess(t *testing.T) {
	var gotMethod string
	var gotContentType string
	var gotBody string
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"id":"WnMMikq","url":"https://mclo.gs/WnMMikq","raw":"https://api.mclo.gs/1/raw/WnMMikq","size":12,"lines":3,"errors":1,"expires":123,"token":"secret-token"}`))
	}))

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	result, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", gotContentType)
	}
	if !strings.Contains(gotBody, `"content":"abc"`) || !strings.Contains(gotBody, `"source":"oaklog"`) {
		t.Fatalf("unexpected request body: %s", gotBody)
	}
	if result.URL != "https://mclo.gs/WnMMikq" || result.Provider != string(ProviderMCLogs) {
		t.Fatalf("unexpected result: %+v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "secret-token") {
		t.Fatalf("token leaked into result json: %s", string(encoded))
	}
}

func TestMCLogsUploadAPIError(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":"bad request"}`))
	}))

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestMCLogsUploadNon2xx(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":"bad request"}`))
	}))

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected non-2xx error, got %v", err)
	}
}

func TestMCLogsUploadMalformedJSON(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true`))
	}))

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil {
		t.Fatal("expected malformed json error")
	}
}

func TestMCLogsUploadEmptyURL(t *testing.T) {
	client := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"id":"abc"}`))
	}))

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil || !strings.Contains(err.Error(), "empty url") {
		t.Fatalf("expected empty url error, got %v", err)
	}
}

func TestMCLogsUploadCapsResponseBodyReads(t *testing.T) {
	body := &trackingBody{limit: maxMCLogsResponseBody + 4096}
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

	c := MCLogsClient{Endpoint: "http://example.invalid/1/log", HTTPClient: client}
	_, err := c.Upload(context.Background(), UploadRequest{Content: []byte("abc"), Source: "oaklog"})
	if err == nil {
		t.Fatal("expected upload error")
	}
	if body.read > maxMCLogsResponseBody {
		t.Fatalf("expected capped read, got %d bytes", body.read)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			return rr.Result(), nil
		}),
	}
}

type trackingBody struct {
	limit int64
	read  int64
}

func (b *trackingBody) Read(p []byte) (int, error) {
	if b.read >= b.limit {
		return 0, io.EOF
	}
	remaining := b.limit - b.read
	if int64(len(p)) > remaining {
		p = p[:int(remaining)]
	}
	for i := range p {
		p[i] = 'x'
	}
	b.read += int64(len(p))
	if b.read >= b.limit {
		return len(p), io.EOF
	}
	return len(p), nil
}

func (b *trackingBody) Close() error {
	return nil
}
