package oaklog

import "context"

type Provider string

const ProviderMCLogs Provider = "mclogs"
const ProviderPastebin Provider = "pastebin"

type UploadRequest struct {
	Content []byte
	Source  string
}

type UploadResult struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	URL      string `json:"url"`
	Raw      string `json:"raw,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Lines    int64  `json:"lines,omitempty"`
	Errors   int64  `json:"errors,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
}

type Uploader interface {
	Upload(ctx context.Context, req UploadRequest) (UploadResult, error)
}
