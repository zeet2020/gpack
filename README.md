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
gpack → wails3 init (scaffold) → render main.go → fetch/convert icon
      → go mod tidy → [wails3 generate bindings] → go build → copy binary to --out
```

It rides Wails' own project scaffolding and only injects the generated `main.go`, a frontend
shell, and icons — so it stays close to upstream as Wails evolves.

## Requirements

- **Go ≥ 1.25** (Wails v3 requires it; `GOTOOLCHAIN=auto` fetches it automatically).
- **[wails3](https://v3.wails.io) + [task](https://taskfile.dev)** on `$GOPATH/bin`
  (gpack locates them by absolute path, so they don't need to be on `PATH`):
  ```bash
  go install github.com/go-task/task/v3/cmd/task@latest
  # Linux (webkit2gtk-4.1): add -tags gtk3
  go install -tags gtk3 github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.98
  ```
- **Platform webview**, needed to *run* built apps:
  - Linux: `gtk3` + `webkit2gtk-4.1` (`libgtk-3-dev libwebkit2gtk-4.1-dev`)
  - macOS: WKWebView (built in) · Windows: WebView2 (built in)

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
| `--user-agent` | platform default | UA override (JS-level, best-effort) |
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
| `--debug` | false | Enable DevTools |
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
- `--user-agent` is a JS override; it does not change the HTTP `User-Agent` header.
- `--incognito`, `--proxy-url`, and `--activation-shortcut` are best-effort and may warn.
- `multi_window` / `new_window` from Pake configs are not supported (single primary window).

## License

[MIT](./LICENSE)
