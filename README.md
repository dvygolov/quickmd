<p align="center">
  <img src="icon/quickmd-1024.png" width="140" alt="QuickMD icon">
</p>

<h1 align="center">QuickMD</h1>

<p align="center">
  A fast native Markdown editor for Windows — one small Go binary, no Electron, no browser.
</p>

<p align="center">
  <img src="docs/screenshot.png" alt="QuickMD screenshot">
</p>

## Why

Markdown editors are usually Electron apps that take seconds to open. QuickMD is a
single ~2 MB Win32 executable written in Go: the window is ready in **~130 ms**.

## Features

- Split view: raw Markdown on the left, live rendered preview on the right.
- YAML front matter is shown as a metadata card above the document.
- Headings, nested and ordered lists, task lists, block quotes, fenced code,
  tables, links, `**bold**`, `*italic*`, `~~strike~~`, inline code.
- Dark theme, DPI-aware, Consolas for the source and Segoe UI for the preview.
- Autosave ~1 s after the last edit, plus `Ctrl+S` and save-on-close.
- Atomic writes (temporary file + rename).
- No runtime dependencies: a single `.exe`.

## Build

Requires Go 1.24+.

```bash
./build.sh
```

or:

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" -o quickmd.exe .
```

The icon is embedded from `icon/quickmd.ico` through the committed
`rsrc_windows_amd64.syso`. To regenerate it:

```bash
go install github.com/akavel/rsrc@latest
rsrc -ico icon/quickmd.ico -arch amd64 -o rsrc_windows_amd64.syso
```

## Usage

```bat
quickmd.exe "D:\notes\post.md"
```

From WSL:

```bash
./quickmd.exe '\\wsl.localhost\Ubuntu-24.04\root\notes\post.md'
```

## BB integration

The editor is also wired into the
[bb-plugin-markdown-open](https://github.com/dvygolov) BB plugin: any `.md` file
opened in BB gets a **«Редактировать»** button that launches QuickMD with the
file's Windows path.

## Limitations

- No syntax highlighting in the source pane yet — plain monospace.
- Raw HTML is shown as code in the preview: without a browser engine it cannot
  be rendered.
- The preview is pasted as RTF through the clipboard (the only way RichEdit
  imports it); the previous clipboard text is restored after 600 ms.
- Two windows editing the same file do not sync: the last save wins.
- The window stays on top while it is the active window; Windows does not let a
  background process take the foreground otherwise.

## Layout

```
main.go      window, RichEdit panes, autosave, launch
win32.go     Win32 API bindings
markdown.go  Markdown and front matter parsing
rtf.go       RTF generation for the preview
icon/        source icon and .ico
build.sh     one-command build
```
