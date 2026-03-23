# lazycut

[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev)
[![FFmpeg](https://img.shields.io/badge/requires-ffmpeg-007808?logo=ffmpeg)](https://ffmpeg.org)
[![Chafa](https://img.shields.io/badge/requires-chafa-4B0082)](https://hpjansson.org/chafa/)
[![License](https://img.shields.io/github/license/emin-ozata/lazycut)](LICENSE)
[![Release](https://img.shields.io/github/v/release/emin-ozata/lazycut)](https://github.com/emin-ozata/lazycut/releases)

Terminal-based video trimming tool. Mark in/out points and export trimmed clips with aspect ratio control.

![lazycut demo](media/demo.gif)

## Install

### macOS

```bash
brew tap emin-ozata/homebrew-tap
brew install lazycut
```

### Windows

Download the latest Windows binary from the [releases page](https://github.com/emin-ozata/lazycut/releases):
- `lazycut_X.X.X_windows_amd64.zip`

Extract and add to your PATH, or run directly.

**Dependencies:**
- ffmpeg: `winget install ffmpeg` or download from [ffmpeg.org](https://ffmpeg.org)

### Build from source

Or build from source:
```bash
git clone https://github.com/emin-ozata/lazycut.git
cd lazycut
go build
./lazycut video.mp4
```

## Usage

```
lazycut <video-file>
```

### Flags

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show help |
| `-v`, `--version` | Show version |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `h` / `l` | Seek ±1s |
| `H` / `L` | Seek ±5s |
| `,` / `.` | Seek ±1 frame |
| `0` | Go to start |
| `G` / `$` | Go to end |
| `i` / `o` | Set in/out points |
| `p` | Preview selection |
| `d` / `Esc` | Clear selection |
| `Enter` | Export |
| `u` | Undo last trim change |
| `m` | Toggle mute |
| `?` | Help |
| `q` | Quit |

Repeat counts work for seek controls: `5l` = seek forward 5 seconds, `10.` = step forward 10 frames.
