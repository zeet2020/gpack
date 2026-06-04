package builder

import (
	"fmt"
	"strings"

	"gpack/internal/config"
)

// shortcutsJS wires the common browser keyboard shortcuts (back/forward/reload/zoom)
// that a native wrapper is expected to provide. Injected unless disabled.
const shortcutsJS = `(function(){document.addEventListener('keydown',function(e){
var mod=e.metaKey||e.ctrlKey;if(!mod)return;
if(e.key==='[')        {e.preventDefault();history.back();}
else if(e.key===']')   {e.preventDefault();history.forward();}
else if(e.key==='r')   {e.preventDefault();location.reload();}
else if(e.key==='-')   {e.preventDefault();document.body.style.zoom=(parseFloat(document.body.style.zoom||1)-0.1);}
else if(e.key==='='||e.key==='+'){e.preventDefault();document.body.style.zoom=(parseFloat(document.body.style.zoom||1)+0.1);}
});})();`

// forceNavJSFmt keeps navigation inside the webview. %s is a JS regex literal (or
// "null"): URLs matching it stay internal; others have their default action blocked.
const forceNavJSFmt = `(function(){var re=%s;function internal(u){try{return !re||re.test(u);}catch(e){return true;}}
var _open=window.open;window.open=function(u){if(internal(u)){location.href=u;return null;}return _open.apply(window,arguments);};
document.addEventListener('click',function(e){var a=e.target&&e.target.closest?e.target.closest('a'):null;
if(a&&a.href&&!internal(a.href)){e.preventDefault();}},true);})();`

// findBarJS adds a minimal Cmd/Ctrl+F in-page find bar (no native find API in v3).
const findBarJS = `(function(){if(window.__gpackFind)return;window.__gpackFind=1;
var bar=document.createElement('div');bar.style.cssText='position:fixed;top:8px;right:8px;z-index:2147483647;display:none;background:#fff;color:#000;border:1px solid #888;border-radius:6px;padding:4px 6px;box-shadow:0 2px 8px rgba(0,0,0,.3);font:13px sans-serif';
var inp=document.createElement('input');inp.placeholder='Find';inp.style.cssText='border:none;outline:none;font:13px sans-serif';bar.appendChild(inp);document.documentElement.appendChild(bar);
function show(){bar.style.display='block';inp.focus();inp.select();}
function hide(){bar.style.display='none';}
document.addEventListener('keydown',function(e){var mod=e.metaKey||e.ctrlKey;
if(mod&&e.key==='f'){e.preventDefault();show();}
else if(e.key==='Escape'){hide();}});
inp.addEventListener('keydown',function(e){if(e.key==='Enter'){try{window.find(inp.value,false,e.shiftKey,true);}catch(_){}}});
})();`

// BuildInject assembles the JS and CSS to inject into every page load for the given
// target OS. CSS is returned separately so it can go into WebviewWindowOptions.CSS,
// which Wails wraps in a <style> tag on each load.
func BuildInject(cfg *config.AppConfig, targetOS string) (js, css string) {
	var jsParts, cssParts []string

	if ua := uaForOS(cfg, targetOS); ua != "" {
		jsParts = append(jsParts, fmt.Sprintf(
			"try{Object.defineProperty(navigator,'userAgent',{get:function(){return %q;}});}catch(e){}", ua))
	}

	if !cfg.Window.DisabledWebShortcuts {
		jsParts = append(jsParts, shortcutsJS)
	}

	if cfg.Window.ForceInternalNav {
		jsParts = append(jsParts, fmt.Sprintf(forceNavJSFmt, jsRegexLiteral(internalPattern(cfg))))
	}

	if cfg.Window.EnableFind {
		jsParts = append(jsParts, findBarJS)
	}

	if cfg.Window.DarkMode {
		cssParts = append(cssParts, ":root{color-scheme:dark;}")
	}

	for _, s := range cfg.Inject {
		if looksLikeCSS(s) {
			cssParts = append(cssParts, strings.TrimPrefix(strings.TrimSpace(s), "/*css*/"))
		} else {
			jsParts = append(jsParts, s)
		}
	}

	return strings.Join(jsParts, "\n"), strings.Join(cssParts, "\n")
}

func uaForOS(cfg *config.AppConfig, targetOS string) string {
	switch targetOS {
	case "darwin":
		return cfg.UserAgent.MacOS
	case "windows":
		return cfg.UserAgent.Windows
	default:
		return cfg.UserAgent.Linux
	}
}

func internalPattern(cfg *config.AppConfig) string {
	if cfg.Window.InternalURLRegex != "" {
		return cfg.Window.InternalURLRegex
	}
	return cfg.SafeDomain
}

// jsRegexLiteral turns a regex source string into a JS regex literal, or "null"
// when empty (meaning "everything is internal").
func jsRegexLiteral(pattern string) string {
	if strings.TrimSpace(pattern) == "" {
		return "null"
	}
	return "/" + strings.ReplaceAll(pattern, "/", `\/`) + "/"
}

// looksLikeCSS is a cheap heuristic: an explicit /*css*/ marker, or a rule-like
// body with no obvious JS tokens.
func looksLikeCSS(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "/*css*/") {
		return true
	}
	hasRule := strings.Contains(s, "{") && strings.Contains(s, "}") && strings.Contains(s, ":")
	hasJS := strings.Contains(s, "function") || strings.Contains(s, "=>") ||
		strings.Contains(s, "var ") || strings.Contains(s, "const ") || strings.Contains(s, "document.")
	return hasRule && !hasJS
}
