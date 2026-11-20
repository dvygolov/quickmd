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
single ~2 MB Win32 executable written in Go: the window is ready in **~150 ms**.

## Features

- Split view: raw Markdown on the left, live rendered preview on the right.
- Two-way sync: typing in the preview writes the change back into the Markdown
  source; typing in the source re-renders the preview.
- Scroll sync: scrolling the preview moves the source to the matching part of the document.
- Main menu: **File** (Open, Save, Save As, Exit), **Edit** (Undo, Redo, Cut,
  Copy, Paste, Select All), **Help** (About).
- Rendered preview: headings, lists, tables, block quotes, fenced code, links,
  inline code, YAML front matter as a metadata card.
- Inline images: `![alt](picture.png)` renders in the preview. PNG is embedded
  as-is; JPEG/GIF are converted to PNG. Relative paths resolve against the
  Markdown file, `%20` and absolute Windows paths work, width is capped at
  560 px, and the encoded image is cached until the file changes.
- Telegram Desktop formatting shortcuts:
  - `Ctrl+B` — bold (`**text**`)
  - `Ctrl+I` — italic (`_text_`)
  - `Ctrl+U` — underline (`__text__`)
  - `Ctrl+Shift+M` — monospace (`` `text` ``)
  - `Ctrl+Shift+X` — strikethrough (`~~text~~`)
  - `Ctrl+Shift+P` — spoiler (`||text||`)
- `Ctrl+Z` / `Ctrl+Y` — undo and redo (snapshot-based, because RichEdit's own undo
  is lost when the preview is re-rendered).
- Autosave ~1 s after the last edit, `Ctrl+S`, save on close, atomic writes.
- Dark theme, DPI-aware, embedded icon, no runtime dependencies.

## Build

Requires Go 1.24+.

```bash
./build.sh
```

The icon is embedded from `icon/quickmd.ico` through the committed
`rsrc_windows_amd64.syso`. To regenerate it:

```bash
go install github.com/akavel/rsrc@latest
rsrc -ico icon/quickmd.ico -arch amd64 -o rsrc_windows_amd64.syso
```

## Install as the default Markdown editor

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1 -Source .\quickmd.exe
```

The script copies QuickMD to `%LOCALAPPDATA%\Programs\QuickMD`, registers the
`QuickMD.md` ProgId, adds it to "Open with" for `.md`, `.markdown` and `.mdx`, and
registers it in Windows Default Apps. Windows protects the final default choice:
the script opens the Default Apps page, where QuickMD has to be selected once.

## Usage

```bat
quickmd.exe "D:\notes\post.md"
```

From WSL:

```bash
./quickmd.exe '\\wsl.localhost\Ubuntu-24.04\root\notes\post.md'
```

## BB integration

The editor is wired into the
[bb-plugin-markdown-open](https://github.com/dvygolov/bb-plugin-markdown-open) BB
plugin: any `.md` file opened in BB has a **«Редактировать»** button that launches
QuickMD with the file's Windows path.

## Limitations

- The preview is RichEdit RTF, not a browser: raw HTML is shown as code and some
  MarkText typography (exact line-height, full-width blocks) differs.
- The preview is pasted through the clipboard (RichEdit imports RTF only that
  way); the previous clipboard text is restored after 600 ms.
- Undo is snapshot-based: a quick burst of typing is undone as one chunk.

## Layout

```
main.go      window, split editor/preview, autosave, undo
menu.go      Win32 main menu and Open/Save dialogs
sync.go      preview ↔ source mapping and two-way edits
win32.go     Win32 API bindings
markdown.go  Markdown and front matter parsing
rtf.go       RTF generation for the rendered preview
shortcuts.go Telegram-style formatting hotkeys
icon/        source icon and .ico
build.sh     one-command build
```
