# Security Policy

## Supported versions

Only the latest release receives fixes. Older tags are not patched.

## Reporting a vulnerability

Report privately through GitHub: open the
[Security tab](https://github.com/ozemin/lazycut/security/advisories/new) and
choose **Report a vulnerability**. Please do not open a public issue for
security reports.

Include the lazycut version, your OS, and the steps needed to reproduce the
issue. You can expect an initial response within 7 days.

## Scope

lazycut is a local terminal application that invokes `ffmpeg`, `ffprobe`,
`ffplay`, and `chafa` as subprocesses on files the user selects. It opens no
network listeners and runs no remote code.

**In scope**

- Argument or path handling that lets a crafted file name influence the
  subprocess command line.
- Unsafe handling of temporary files or export output paths.
- Anything that turns opening a video file into code execution.

**Out of scope**

- Vulnerabilities in `ffmpeg`, `chafa`, or other external binaries — report
  those to their maintainers.
- Crashes or malformed output caused by corrupt input files, unless they lead
  to code execution.
