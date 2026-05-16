# Oaklog

Oaklog is a small Go CLI for uploading Minecraft server logs and printing a shareable URL. Default backend is mclo.gs. Pastebin is available with `oaklog pastebin`.

By default Oaklog prints only the URL so it is easy to pipe into scripts, chat messages, or shell aliases.

## Install

```bash
go install github.com/rannday/oaklog/cmd/oaklog@latest
```

## Build

```bash
go build ./cmd/oaklog
```

## Usage

```bash
oaklog latest.log
```

Explicit mclo.gs:

```bash
oaklog mclogs latest.log
```

Explicit Pastebin:

```bash
oaklog pastebin latest.log
```

Public Pastebin:

```bash
oaklog pastebin --public latest.log
```

Unlisted Pastebin:

```bash
oaklog pastebin --unlisted latest.log
```

Output:

```text
https://mclo.gs/abc1234
```

JSON output:

```bash
oaklog -j latest.log
```

Pastebin JSON:

```bash
oaklog pastebin -j latest.log
```

Example output:

```json
{
  "provider": "mclogs",
  "id": "abc1234",
  "url": "https://mclo.gs/abc1234",
  "raw": "https://api.mclo.gs/1/raw/abc1234",
  "size": 157369,
  "lines": 1201,
  "errors": 8,
  "expires": 1777373979
}
```

stdin:

```bash
cat latest.log | oaklog -
```

Pastebin stdin:

```bash
cat latest.log | oaklog pastebin -
```

## Options

- `oaklog [flags] <log-file|->` default mclo.gs mode
- `oaklog mclogs [flags] <log-file|->` explicit mclo.gs mode
- `oaklog pastebin [flags] <log-file|->` Pastebin mode
- `-s, --source` set source label/title sent to upload provider
- `-t, --timeout` set HTTP timeout using Go duration syntax
- `-j, --json` print machine-readable JSON output
- `--public` create public Pastebin paste
- `--unlisted` create unlisted Pastebin paste
- `-v, --version` print Oaklog version and exit
- `-h, --help` print help

## Config

Pastebin uses repo-root `.env` or process environment.

```bash
cp .env.example .env
```

Then set:

```bash
PASTEBIN_API=...
```

`PASTEBIN_API` is required only for `oaklog pastebin`.
Pastebin defaults to public. Use `--unlisted` for unlisted pastes.
Release uploads use repo-root `.env.github-releases` or `GITHUB_RELEASES_ENV_FILE`.

## Limits

- Max content size: 10 MiB
- mclo.gs line limit: 25,000 lines
- Oaklog truncates to the last 25,000 lines before mclo.gs upload when needed
- Pastebin does not use the mclo.gs line truncation

## Release builds

Build release archives and checksums:

```bash
go tool go-build-bin -v 0.1.0 -c
```

The external build tool creates archives for Windows, Linux, and macOS and writes `checksums.txt`.

Upload draft release:

```bash
go tool go-github-releases -v 0.1.0 --draft
```

Recommended flow:

```bash
cp .env.example .env
printf 'GITHUB_RELEASES_PAT=\nGITHUB_RELEASES_REPO=rannday/oaklog\nGITHUB_RELEASES_ARTIFACT_DIR=tmp/release\n' > .env.github-releases
go test ./...
go tool go-build-bin -v 0.1.0 -c
go tool go-github-releases -v 0.1.0 --draft --dry-run
go tool go-github-releases -v 0.1.0 --draft
```

Pin the external tool in `go.mod` with:

```bash
go get -tool github.com/rannday/go-build-bin/cmd/go-build-bin@latest
go get -tool github.com/rannday/go-github-releases/cmd/go-github-releases@latest
```

`go-github-releases` loads `.env.github-releases` automatically from the current directory.

Set these in your shell when you want release uploads without the env file:

```bash
export GITHUB_RELEASES_PAT=...
export GITHUB_RELEASES_REPO=rannday/oaklog
export GITHUB_RELEASES_ARTIFACT_DIR=tmp/release
```

PowerShell:

```powershell
$env:GITHUB_RELEASES_PAT="..."
$env:GITHUB_RELEASES_REPO="rannday/oaklog"
$env:GITHUB_RELEASES_ARTIFACT_DIR="tmp/release"
```

`GITHUB_RELEASES_PAT` is required only for non-dry-run `go-github-releases` uploads. The token needs release/content write access to `rannday/oaklog`.
`.env.github-releases` is ignored already in `.gitignore`, so release config stays local.

`PASTEBIN_API` may be read from repo-root `.env` by `oaklog pastebin`.

## Scope

Oaklog supports mclo.gs by default and Pastebin via `oaklog pastebin`.
