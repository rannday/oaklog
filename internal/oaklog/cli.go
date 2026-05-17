package oaklog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxLogSize = 10 << 20
const maxSourceLength = 64
const maxMCLogsLines = 25_000

const mcLogsTruncationMarker = "[oaklog] log truncated to last 25000 lines due to mclo.gs line limit: "

var newUploader = func(cfg providerConfig) Uploader {
	if cfg.Provider == "" {
		cfg.Provider = ProviderMCLogs
	}
	return newUploaderForConfig(cfg)
}

type providerConfig struct {
	Provider        Provider
	Timeout         time.Duration
	PastebinAPI     string
	PastebinPrivate string
}

func newUploaderForConfig(cfg providerConfig) Uploader {
	switch cfg.Provider {
	case ProviderPastebin:
		return &PastebinClient{
			HTTPClient: &http.Client{Timeout: cfg.Timeout},
			APIKey:     cfg.PastebinAPI,
			Private:    cfg.PastebinPrivate,
		}
	default:
		return &MCLogsClient{
			HTTPClient: &http.Client{Timeout: cfg.Timeout},
		}
	}
}

type cliOptions struct {
	Provider        Provider
	Source          string
	Timeout         time.Duration
	JSON            bool
	LogPath         string
	PastebinAPI     string
	PastebinAPIFile string
	PastebinPrivate string
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return run(ctx, args, os.Stdin, stdout, stderr)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	opts, handled, err := parseCLI(args, stdout)
	if handled || err != nil {
		return err
	}

	if opts.Provider == ProviderPastebin {
		apiKey, err := resolvePastebinAPIKey(opts)
		if err != nil {
			return err
		}
		opts.PastebinAPI = apiKey
	}

	content, err := readLogInput(stdin, opts.LogPath)
	if err != nil {
		return err
	}
	if opts.Provider == ProviderMCLogs {
		content = prepareLogContent(content)
	}

	client := newUploader(providerConfig{
		Provider:        opts.Provider,
		Timeout:         opts.Timeout,
		PastebinAPI:     opts.PastebinAPI,
		PastebinPrivate: opts.PastebinPrivate,
	})
	result, err := client.Upload(ctx, UploadRequest{Content: content, Source: opts.Source})
	if err != nil {
		return err
	}

	if opts.JSON {
		return writeJSON(stdout, result)
	}
	_, err = fmt.Fprintln(stdout, result.URL)
	return err
}

func parseCLI(args []string, stdout io.Writer) (cliOptions, bool, error) {
	if len(args) == 0 {
		return cliOptions{}, false, errors.New("exactly one log file path is required")
	}

	if hasLegacyProviderFlag(args, "--mclogs") {
		return cliOptions{}, false, errors.New("--mclogs has been replaced by: oaklog mclogs <log-file|->")
	}
	if hasLegacyProviderFlag(args, "--pastebin") {
		return cliOptions{}, false, errors.New("--pastebin has been replaced by: oaklog pastebin <log-file|->")
	}

	if hasAny(args, "-h", "--help") {
		switch args[0] {
		case "mclogs":
			printMCLLogsHelp(stdout)
		case "pastebin":
			printPastebinHelp(stdout)
		default:
			printTopLevelHelp(stdout)
		}
		return cliOptions{}, true, nil
	}

	if hasAny(args, "-v", "--version") {
		printVersion(stdout)
		return cliOptions{}, true, nil
	}

	switch args[0] {
	case "mclogs":
		return parseProviderCommand(ProviderMCLogs, args[1:], stdout)
	case "pastebin":
		return parseProviderCommand(ProviderPastebin, args[1:], stdout)
	}

	if strings.HasPrefix(args[0], "-") {
		return parseDefaultCommand(args, stdout)
	}

	if len(args) != 1 {
		return cliOptions{}, false, errors.New("exactly one log file path is required")
	}

	return cliOptions{
		Provider: ProviderMCLogs,
		Source:   "oaklog",
		Timeout:  30 * time.Second,
		LogPath:  args[0],
	}, false, nil
}

func parseDefaultCommand(args []string, stdout io.Writer) (cliOptions, bool, error) {
	fs := flag.NewFlagSet("oaklog", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var source string
	var timeout string
	var jsonOut bool

	fs.StringVar(&source, "source", "oaklog", "source label/title sent to upload provider")
	fs.StringVar(&timeout, "timeout", "30s", "HTTP timeout")
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON output")

	if err := fs.Parse(normalizeShortFlags(args)); err != nil {
		return cliOptions{}, false, err
	}
	if fs.NArg() != 1 {
		return cliOptions{}, false, errors.New("exactly one log file path is required")
	}

	timeoutDur, err := time.ParseDuration(timeout)
	if err != nil {
		return cliOptions{}, false, fmt.Errorf("invalid --timeout value %q: %w", timeout, err)
	}

	source, err = validateSourceLabel(source)
	if err != nil {
		return cliOptions{}, false, err
	}

	return cliOptions{
		Provider: ProviderMCLogs,
		Source:   source,
		Timeout:  timeoutDur,
		JSON:     jsonOut,
		LogPath:  fs.Arg(0),
	}, false, nil
}

func parseProviderCommand(provider Provider, args []string, stdout io.Writer) (cliOptions, bool, error) {
	fs := flag.NewFlagSet(string(provider), flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var source string
	var timeout string
	var jsonOut bool
	var pastebinAPI string
	var pastebinAPIFile string
	var pastebinPublic bool
	var pastebinUnlisted bool

	fs.StringVar(&source, "source", "oaklog", "source label/title sent to upload provider")
	fs.StringVar(&timeout, "timeout", "30s", "HTTP timeout")
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON output")
	if provider == ProviderPastebin {
		fs.StringVar(&pastebinAPI, "pastebin-api", "", "Pastebin API token")
		fs.StringVar(&pastebinAPIFile, "pastebin-api-file", "", "path to a file containing only the Pastebin API token")
		fs.BoolVar(&pastebinPublic, "public", false, "create a public paste")
		fs.BoolVar(&pastebinUnlisted, "unlisted", false, "create an unlisted paste")
	}

	if err := fs.Parse(normalizeShortFlags(args)); err != nil {
		return cliOptions{}, false, err
	}
	if fs.NArg() != 1 {
		return cliOptions{}, false, errors.New("exactly one log file path is required")
	}

	timeoutDur, err := time.ParseDuration(timeout)
	if err != nil {
		return cliOptions{}, false, fmt.Errorf("invalid --timeout value %q: %w", timeout, err)
	}

	source, err = validateSourceLabel(source)
	if err != nil {
		return cliOptions{}, false, err
	}

	var pastebinPrivate string
	if provider == ProviderPastebin {
		if pastebinPublic && pastebinUnlisted {
			return cliOptions{}, false, errors.New("--public and --unlisted cannot be used together")
		}
		switch {
		case pastebinUnlisted:
			pastebinPrivate = pastebinVisibilityUnlisted
		default:
			pastebinPrivate = pastebinVisibilityPublic
		}
	}

	return cliOptions{
		Provider:        provider,
		Source:          source,
		Timeout:         timeoutDur,
		JSON:            jsonOut,
		LogPath:         fs.Arg(0),
		PastebinAPI:     pastebinAPI,
		PastebinAPIFile: pastebinAPIFile,
		PastebinPrivate: pastebinPrivate,
	}, false, nil
}

func printTopLevelHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  oaklog [flags] <log-file|->")
	fmt.Fprintln(w, "  oaklog <provider> [flags] <log-file|->")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Providers:")
	fmt.Fprintln(w, "  mclogs      upload to mclo.gs")
	fmt.Fprintln(w, "  pastebin    upload to Pastebin")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Default provider:")
	fmt.Fprintln(w, "  mclogs")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -s, --source string      source label/title sent to upload provider (default \"oaklog\")")
	fmt.Fprintln(w, "  -t, --timeout duration   HTTP timeout (default 30s)")
	fmt.Fprintln(w, "  -j, --json               print machine-readable JSON output")
	fmt.Fprintln(w, "  -v, --version            print version and exit")
	fmt.Fprintln(w, "  -h, --help               print help")
}

func printMCLLogsHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  oaklog mclogs [flags] <log-file|->")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Upload Minecraft server logs to mclo.gs.")
	fmt.Fprintln(w, "mclo.gs is default provider for `oaklog <log-file|->`.")
	fmt.Fprintln(w, "This provider truncates to last 25000 lines when needed.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -s, --source string      source label/title sent to upload provider (default \"oaklog\")")
	fmt.Fprintln(w, "  -t, --timeout duration   HTTP timeout (default 30s)")
	fmt.Fprintln(w, "  -j, --json               print machine-readable JSON output")
	fmt.Fprintln(w, "  -h, --help               print help")
}

func printPastebinHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  oaklog pastebin [flags] <log-file|->")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Upload Minecraft server logs to Pastebin.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -s, --source string      paste title/source label (default \"oaklog\")")
	fmt.Fprintln(w, "  -t, --timeout duration   HTTP timeout (default 30s)")
	fmt.Fprintln(w, "  -j, --json               print machine-readable JSON output")
	fmt.Fprintln(w, "      --pastebin-api       Pastebin API token")
	fmt.Fprintln(w, "      --pastebin-api-file  path to a file containing only the Pastebin API token")
	fmt.Fprintln(w, "      --public             create a public paste (default)")
	fmt.Fprintln(w, "      --unlisted           create an unlisted paste")
	fmt.Fprintln(w, "  -h, --help               print help")
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "oaklog %s\n", Version)
}

func hasAny(args []string, values ...string) bool {
	for _, arg := range args {
		for _, value := range values {
			if arg == value {
				return true
			}
		}
	}
	return false
}

func hasLegacyProviderFlag(args []string, flagName string) bool {
	for _, arg := range args {
		if arg == flagName || strings.HasPrefix(arg, flagName+"=") {
			return true
		}
	}
	return false
}

func normalizeShortFlags(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		switch arg {
		case "-h":
			out[i] = "--help"
		case "-v":
			out[i] = "--version"
		case "-j":
			out[i] = "--json"
		case "-s":
			out[i] = "--source"
		case "-t":
			out[i] = "--timeout"
		default:
			out[i] = arg
		}
	}
	return out
}

func validateSourceLabel(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("--source cannot be empty")
	}
	if len(source) > maxSourceLength {
		return "", fmt.Errorf("--source must be %d characters or fewer", maxSourceLength)
	}
	return source, nil
}

const (
	pastebinVisibilityPublic   = "0"
	pastebinVisibilityUnlisted = "1"
)

func readLogInput(stdin io.Reader, path string) ([]byte, error) {
	if path == "-" {
		content, err := io.ReadAll(io.LimitReader(stdin, maxLogSize+1))
		if err != nil {
			return nil, err
		}
		if len(content) == 0 {
			return nil, errors.New("stdin is empty")
		}
		if len(content) > maxLogSize {
			return nil, errors.New("stdin is larger than 10 MiB")
		}
		return content, nil
	}
	return readLogFile(path)
}

func readLogFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log file not found: %s", path)
		}
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("log file must be a regular file: %s", path)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("log file is empty: %s", path)
	}
	if info.Size() > maxLogSize {
		return nil, fmt.Errorf("log file is larger than 10 MiB: %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("log file is empty: %s", path)
	}
	return content, nil
}

func writeJSON(w io.Writer, result UploadResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func prepareLogContent(content []byte) []byte {
	if countLogLines(content) <= maxMCLogsLines {
		return content
	}
	start := startOfLastLines(content, maxMCLogsLines)
	out := make([]byte, 0, len(mcLogsTruncationMarker)+len(content[start:]))
	out = append(out, mcLogsTruncationMarker...)
	out = append(out, content[start:]...)
	return out
}

func countLogLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines
}

func startOfLastLines(content []byte, keep int) int {
	if keep <= 0 || len(content) == 0 {
		return len(content)
	}
	lines := countLogLines(content)
	if lines <= keep {
		return 0
	}
	skip := lines - keep
	newlines := 0
	for i, b := range content {
		if b == '\n' {
			newlines++
			if newlines == skip {
				return i + 1
			}
		}
	}
	return 0
}
