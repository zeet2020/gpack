# gpack

> Turn any URL into a native desktop app with a single command — Go + [Wails v3](https://v3.wails.io).

`gpack` is a CLI that wraps a website (or a local HTML app) in a native window. It accepts a
[Pake](https://github.com/tw93/Pake)-compatible JSON config **or** plain CLI flags, generates a
throwaway Wails v3 project, builds it, and copies out a single native binary. You never write Go
or frontend code.

```bash
gpack https://news.ycombinator.com --name HN --out ./dist
# → ./dist/HN   (native app, no browser chrome)
```

---

## How it works

```
gpack → generate project (main.go + go.mod + frontend shell) → fetch/convert icon
      → go mod tidy → [wails3 generate bindings] → go build → copy binary to --out
```

The generated project is a single `main.go` against the pinned Wails module
(`v3.0.0-beta.10`), so there is no scaffolding to keep in sync — one const in
`internal/builder/bootstrap.go` moves the whole toolchain.

## Requirements

gpack bootstraps its Go-side toolchain itself — **no `wails3` or `task` install needed**. To build
apps you only need:

- **A C compiler** (`gcc`/`clang`) — cgo links the webview.
- **System webview dev libraries** (needed to build *and* run):
  - **Linux:** gtk3 + webkit2gtk-4.1 → `sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev`
    (or run `gpack … --install-deps` to attempt it). These are C libraries and cannot be
    auto-built — same floor as Tauri/Electron.
  - **macOS:** WKWebView (built in) · **Windows:** WebView2 (preinstalled on Win 11)

Handled automatically: the **Go toolchain** (uses your `go` if ≥1.21 via `GOTOOLCHAIN=auto`, else
downloads one into the gpack cache, verified against the SHA-256 in go.dev's release index before
it is unpacked), the **Wails v3 module**, and typed **bindings** (`go run`, local-file only).

## Install

```bash
go install ./cmd/gpack        # → $GOPATH/bin/gpack
# or
go build -o gpack ./cmd/gpack
```

## Usage

```
gpack [url] [flags]
gpack --config <pake.json> [flags]
```

**From a URL:**
```bash
gpack https://weekly.tw93.fun/en --name Weekly --hide-title-bar --out ./dist
```

**From a Pake JSON config** (existing Pake users can reuse their files):
```bash
gpack --config examples/weekly.json --name Weekly --out ./dist
```
Paths inside a config (`inject`, `system_tray_path`, and a `url_type: "local"` url) resolve
relative to the config file, not the working directory. `inject` entries are file paths —
`.css` files land in the stylesheet slot, everything else is injected as JS.

**A local HTML app** (gets a JS→Go native bridge — see [Bound methods](#bound-methods)):
```bash
gpack ./mysite --use-local-file --name MySite --out ./dist
```

Inspect the resolved config without building:
```bash
gpack --config examples/weekly.json --dry-run
```

## Options

### Window
| Flag | Default | Description |
|---|---|---|
| `--name` | derived from host | App + binary name |
| `--width` / `--height` | 1200 / 780 | Window size |
| `--min-width` / `--min-height` | 0 | Minimum size (0 = none) |
| `--zoom` | 100 | Page zoom (50–200), native |
| `--hide-title-bar` | false | Frameless window |
| `--fullscreen` / `--maximize` | false | Initial window state |
| `--resizable` | true | Allow resizing |
| `--always-on-top` | false | Float above other windows |
| `--dark-mode` | false | Dark background + `color-scheme` |
| `--title` | name | Title-bar text |

### Icons
| Flag | Default | Description |
|---|---|---|
| `--icon` | auto favicon | Icon path or URL (PNG/JPG/GIF → 256px) |
| `--system-tray-icon` | app icon | Separate tray icon |

When `--icon` is omitted, gpack auto-fetches the site's favicon.

### Web behaviour
| Flag | Default | Description |
|---|---|---|
| `--user-agent` | unset | UA override (JS-level only — see [Limitations](#limitations)) |
| `--disabled-web-shortcuts` | false | Drop built-in keyboard shortcuts |
| `--enable-find` | false | In-page find bar (Cmd/Ctrl+F) |
| `--force-internal-navigation` | false | Keep links inside the window |
| `--safe-domain` | "" | Regex of URLs that stay internal |
| `--enable-drag-drop` | false | Native file drag-and-drop |
| `--inject` | — | JS/CSS file(s) injected on every load (repeatable) |
| `--incognito` | false | Best-effort ephemeral session |

### System integration
| Flag | Default | Description |
|---|---|---|
| `--show-system-tray` | false | Tray icon with Show/Quit |
| `--hide-on-close` | false | Close hides to tray instead of quitting |
| `--start-to-tray` | false | Launch hidden (needs tray) |
| `--multi-instance` | false | Allow more than one running copy |

### Build
| Flag | Default | Description |
|---|---|---|
| `--out` | `.` | Output directory for the binary |
| `--app-version` | 1.0.0 | App version |
| `--debug` | false | Enable DevTools; stream build output live |
| `--use-local-file` | false | Treat the argument as a local dir/file and embed it |
| `--config` | "" | Pake JSON config file |
| `--keep-tmp` | false | Keep the generated project (debugging) |
| `--dry-run` | false | Print resolved config, don't build |

`gpack --help` lists every flag.

## Bound methods

For remote URLs, gpack drives the app entirely from the Go side (window, tray, JS/CSS injection) —
the page itself cannot call back into Go (the binding bridge is locked to the app's own origin).

With **`--use-local-file`**, content is served from the app origin, so the page can call native
methods via generated bindings:

```js
window.go.main.GpackBridge.OpenExternal("https://example.com");
window.go.main.GpackBridge.Minimize();
window.go.main.GpackBridge.ToggleMaximize();
window.go.main.GpackBridge.Fullscreen();
window.go.main.GpackBridge.Reload();
window.go.main.GpackBridge.Quit();
```

## GitHub Actions

- **`.github/workflows/ci.yml`** — gofmt / vet / build / test the tool.
- **`.github/workflows/build-app.yml`** — *Run workflow* with a URL or config to build native
  artifacts on macOS, Windows and Linux, uploaded as downloadable artifacts.

## Limitations

- Builds for the **current OS only** (Wails uses cgo; cross-compile is not wired). Use the
  Build-App workflow to produce all three platforms.
- Output is a raw binary, not a `.deb`/`.dmg`/`.msi` installer yet.
- `--user-agent` is a JS override applied after load; it does not change the HTTP `User-Agent`
  header, and there is no default UA — unset means the webview's own.
- These Pake CLI flags are **parsed but ignored** (using one prints a warning): `--targets`,
  `--multi-arch`, `--iterative-build`, `--keep-binary`, `--installer-language`, `--wasm`.
- `--incognito`, `--proxy-url`, and `--activation-shortcut` are best-effort and may warn.
- `multi_window` / `new_window` from Pake configs are not supported (single primary window).

## License

[MIT](./LICENSE)
