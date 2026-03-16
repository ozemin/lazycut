# lazycut

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
- chafa: Install via [Scoop](https://scoop.sh): `scoop install chafa`

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

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `h` / `l` | Seek ±1s |
| `H` / `L` | Seek ±5s |
| `i` / `o` | Set in/out points |
| `Enter` | Export |
| `?` | Help |
| `q` | Quit |

Repeat counts work: `5l` = seek forward 5 seconds.
