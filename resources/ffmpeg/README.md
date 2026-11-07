Place bundled FFmpeg binaries under OS/arch-specific folders:

- `resources/ffmpeg/darwin-arm64/ffmpeg`
- `resources/ffmpeg/darwin-arm64/ffprobe`
- `resources/ffmpeg/darwin-amd64/ffmpeg`
- `resources/ffmpeg/darwin-amd64/ffprobe`
- `resources/ffmpeg/linux-amd64/ffmpeg`
- `resources/ffmpeg/linux-amd64/ffprobe`
- `resources/ffmpeg/windows-amd64/ffmpeg.exe`
- `resources/ffmpeg/windows-amd64/ffprobe.exe`

If not present, the app falls back to `ffmpeg` and `ffprobe` from system `PATH`.
