# Typora — native Windows Markdown editor

> **Independent project.** This repository is not affiliated with, endorsed by, or the source code of the commercial **Typora** application. It is an independent Markdown editor built from scratch and inspired by the same distraction-free editing idea.

A fast, local-first Windows Markdown editor focused on feeling as immediate as Notepad while adding tabs, automatic persistence, Markdown preview, recovery, and a modern native interface.

**Current version: v4.18.0**

## Highlights

- **Native Windows application** — no Electron, browser wrapper, localhost server, or cloud account.
- **Fast multi-tab editing** with drag-to-reorder, middle-click close, recently closed tabs, and persistent sessions.
- **Seamless saving** — new notes are created automatically in your chosen notes folder and saved continuously.
- **Crash/session recovery** is separate from the real files, so unsaved buffers survive restarts without corrupting the document on disk.
- **Atomic disk writes** and external-change protection.
- **External deletion safety** — deleting an open file in Explorer will not let autosave silently resurrect it.
- **Live Notes sidebar** watching `.md`, `.markdown`, and `.txt` files created, renamed, or deleted in Explorer.
- **Edit / Split / Preview** views.
- **Native Markdown preview** with headings, emphasis, links, quotes, lists, tables, code, and custom-painted rounded fenced-code blocks.
- **Language-aware fenced code** with lightweight syntax coloring and a subtle language label.
- **Dark and light themes**.
- **Native Windows file dialogs** for Open, Folder, and Save As.
- **First-run tutorial** plus an in-app **F1** keyboard-shortcut sheet.

## Saving model

Typora uses two persistence layers:

```text
Typing
  ├─ short debounce → private recovery snapshot
  └─ short debounce → atomic save to the real note file
```

The recovery copy does not replace the actual document. `Ctrl+S` flushes immediately, while `Ctrl+Shift+S` opens the native Windows **Save As** dialog.

New notes begin as `Untitled 01`, `Untitled 02`, and so on — never just `Untitled` — and the name is selected immediately so you can type a better title.

## External file changes

The Notes sidebar watches the selected root notes directory. Creating, deleting, or renaming a supported file in Explorer is reflected automatically.

If an **open** file is deleted externally, its tab remains available as a recovery buffer but background saving to the deleted path is suspended. Typora will not recreate the file unless you explicitly choose to restore it with `Ctrl+S`.

## Fenced code blocks

Inline code remains compact:

```markdown
Use `RabbitMQ` here.
```

Fenced blocks are rendered as native rounded cards with syntax coloring:

````markdown
```json
{
  "order_id": 98124,
  "customer_id": 918,
  "total": 74.29
}
```
````

Supported language tags include common aliases for Python, JavaScript, TypeScript, Go, Java, C/C++, C#, Rust, Swift, Kotlin, SQL, Bash, PowerShell, JSON, YAML, TOML, HTML/XML, and CSS.

## Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| `Ctrl+N` | New note |
| `Ctrl+O` | Open file |
| `Ctrl+S` | Save / flush now |
| `Ctrl+Shift+S` | Save As |
| `Ctrl+W` | Close tab |
| `Ctrl+Shift+T` | Reopen recently closed tab |
| `Ctrl+Tab` | Next tab |
| `Ctrl+Shift+Tab` | Previous tab |
| `F2` | Rename managed note |
| `F1` | Keyboard shortcuts |
| `Esc` | Dismiss overlays/dialogs |

The app also supports middle-click-to-close tabs and right-click context menus for tabs and Notes-sidebar items.

## Build from source

### Requirements

- Windows 10/11
- Go 1.23+
- Python 3
- Pillow (`python -m pip install pillow`) for generating the multi-size Windows icon

### Build

```bat
build.bat
```

The script generates `Typora.ico`, compiles the native Windows executable, and embeds the icon into the final `Typora.exe`.

You can also use the included GitHub Actions workflow; each push to `main` builds a Windows executable and publishes it as a workflow artifact.

## Project layout

```text
main.go            Native Windows UI, tabs, input, commands, app lifecycle
markdown.go        Markdown parsing / preview model
preview_native.go  Native preview formatting and custom code-block rendering
storage.go         Config, sessions, recovery, atomic persistence
async_io.go        Background save/recovery workers
folder_watch.go    Live notes-directory watcher
dialogs.go         Native Windows dialogs
embed_icon.py      PE icon-resource embedding
make_icon.py       Multi-size ICO generation from the PNG logo
assets/            Logo source
```

## Current limitations / roadmap

- The Notes sidebar currently watches only the selected folder root. Nested directories are intentionally not flattened; the intended future design is a proper collapsible folder tree.
- The native Markdown renderer is still evolving. Complex document features may receive dedicated renderers over time.
- Windows is the only supported platform right now.

## Diagnostics

Runtime log:

```text
%TEMP%\Typora-v4.18.log
```

## License

MIT. See [LICENSE](LICENSE).
