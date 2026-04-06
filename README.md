# lazycut

[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev)
[![FFmpeg](https://img.shields.io/badge/requires-ffmpeg-007808?logo=ffmpeg)](https://ffmpeg.org)
[![Chafa](https://img.shields.io/badge/requires-chafa-4B0082)](https://hpjansson.org/chafa/)
[![License](https://img.shields.io/github/license/ozemin/lazycut)](LICENSE)
[![Release](https://img.shields.io/github/v/release/ozemin/lazycut)](https://github.com/ozemin/lazycut/releases)

Terminal-based video trimming tool. Mark in/out points and export trimmed clips with aspect ratio control.

![lazycut demo](media/demo.gif)

## Install

### macOS

```bash
brew install lazycut
```

### Build from source
Prerequisites: Latest version of Golang + ffmpeg + chafa dev packages.
<details>
<summary>Fedora</summary>

Install latest version of Golang via COPR as recommended [by Fedora docs](https://developer.fedoraproject.org/tech/languages/go/go-installation.html)
```sh
sudo dnf copr enable @go-sig/golang-rawhide
sudo dnf install golang
```

Install ffmpeg and chafa
```sh
sudo dnf install -y ffmpeg chafa chafa-devel
```
</details>

```bash
go install github.com/ozemin/lazycut@latest 
./lazycut video.mp4
```

Requires [FFmpeg](https://ffmpeg.org) and [Chafa](https://hpjansson.org/chafa/).

## Usage

```bash
lazycut video.mp4              # interactive TUI
lazycut video.mp4 --fps 12     # custom preview frame rate
lazycut trim --in 00:01:00 --out 00:02:00 -o out.mp4 video.mp4
lazycut probe video.mp4
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `h` / `l` | Seek ±1s |
| `H` / `L` | Seek ±5s |
| `←` / `→` | Seek ±5s |
| `Shift+←` / `Shift+→` | Seek ±1s |
| `↑` / `↓` | Seek ±1 minute |
| `,` / `.` | Seek ±1 frame |
| `0` | Go to start |
| `G` / `$` | Go to end |
| `i` / `o` | Set in/out points |
| `X` | Remove last section |
| `p` / `P` | Preview all / last section |
| `d` / `Esc` | Clear selection |
| `Enter` | Export |
| `u` | Undo |
| `m` | Toggle mute |
| `?` | Help |
| `q` | Quit |

Vim-style repeat counts: `5l` = seek 5s forward, `10.` = step 10 frames.

## Export

Supports multiple sections per video, separate or concatenated export, and aspect ratio conversion (16:9, 9:16, 1:1, 4:5).

## License

[MIT](LICENSE)
