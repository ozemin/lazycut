# Contributing to lazycut

Thanks for taking the time to contribute.

## Prerequisites

lazycut shells out to external binaries at runtime — they must be on your `PATH`:

| Tool | Used for |
|------|----------|
| `ffmpeg` | frame decoding and export |
| `ffprobe` | reading video properties |
| `ffplay` | audio playback |
| `chafa` | rendering frames as terminal graphics |

The Go toolchain version is pinned in [`go.mod`](go.mod).

```bash
# macOS
brew install go ffmpeg chafa

# Fedora
sudo dnf install -y golang ffmpeg chafa

# Debian / Ubuntu
sudo apt-get install -y golang ffmpeg chafa
```

> Older distributions ship a chafa too old for the symbol set lazycut uses. If
> previews look wrong, check `chafa --version` first.

## Development

```bash
git clone https://github.com/ozemin/lazycut.git
cd lazycut
go mod download

make build          # build ./lazycut
./lazycut video.mp4 # run the TUI
```

## Before opening a pull request

```bash
make check
```

This runs the same gates as CI:

| Target | Command |
|--------|---------|
| `make fmt-check` | `golangci-lint fmt --diff` |
| `make lint` | `golangci-lint run` |
| `make test-race` | `go test -race ./...` |
| `make vuln` | `govulncheck ./...` |

Use `make fmt` to apply formatting (gofumpt + import grouping) automatically.

[golangci-lint](https://golangci-lint.run/docs/welcome/install/) needs to be
installed separately; everything else runs through the Go toolchain.

## Commit messages

This repository follows [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add arrow key seek bindings
fix: keep audio in sync after seeking backwards
refactor: feed chafa PNG frames instead of raw PPM
docs: document the export panel
chore: bump bubbletea
ci: run tests on macOS
```

Release notes are generated from commit messages by GoReleaser, so keep the
subject line meaningful.

## Pull requests

- One logical change per pull request.
- Link the issue it closes (`Closes #123`).
- Describe how you verified the change — terminal emulator and OS matter for
  TUI behaviour, so mention them.
- Keep unrelated refactors and reformatting out of the diff.
