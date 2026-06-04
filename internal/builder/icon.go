package builder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"

	"gpack/internal/config"
)

const iconSize = 256

// ResolveAppIcon returns a 256×256 PNG for the app icon. Resolution order:
//  1. --icon path or URL
//  2. auto-fetched favicon for the target URL's host
//  3. a generated default icon
func ResolveAppIcon(cfg *config.AppConfig) []byte {
	if cfg.Icon != "" {
		if raw, err := loadIconSource(cfg.Icon); err == nil {
			if pngBytes, err := normalizePNG(raw); err == nil {
				return pngBytes
			} else {
				fmt.Fprintln(os.Stderr, "warning: could not decode --icon:", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "warning: could not read --icon:", err)
		}
	}

	if raw, err := fetchFavicon(cfg.Window.URL); err == nil {
		if pngBytes, err := normalizePNG(raw); err == nil {
			return pngBytes
		}
	}

	return defaultIconPNG(cfg.Name)
}

// ResolveTrayIcon returns the tray icon PNG: the configured system_tray_path if
// present, otherwise the app icon.
func ResolveTrayIcon(cfg *config.AppConfig, appIcon []byte) []byte {
	if cfg.SystemTrayPath != "" {
		if raw, err := os.ReadFile(cfg.SystemTrayPath); err == nil {
			if pngBytes, err := normalizePNG(raw); err == nil {
				return pngBytes
			}
		}
	}
	return appIcon
}

func loadIconSource(src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return httpGet(src)
	}
	return os.ReadFile(src)
}

// fetchFavicon retrieves a PNG favicon for the URL's host via Google's favicon
// service (returns a real PNG at the requested size, avoiding an ICO decoder).
func fetchFavicon(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("no host in %q", rawURL)
	}
	endpoint := fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=%d", u.Hostname(), iconSize)
	return httpGet(endpoint)
}

func httpGet(u string) ([]byte, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// normalizePNG decodes any supported raster image and re-encodes it as a square
// 256×256 PNG.
func normalizePNG(raw []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return encodeResizedPNG(img), nil
}

func encodeResizedPNG(src image.Image) []byte {
	dst := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	var buf bytes.Buffer
	_ = png.Encode(&buf, dst)
	return buf.Bytes()
}

// defaultIconPNG draws a simple rounded-tile icon with the app's initial.
func defaultIconPNG(name string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	bg := color.RGBA{0x2b, 0x2f, 0x36, 0xff}
	fg := color.RGBA{0x5b, 0x9d, 0xff, 0xff}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// A centered filled circle as a neutral glyph.
	cx, cy, r := iconSize/2, iconSize/2, iconSize/4
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fg)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
