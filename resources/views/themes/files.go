/*
|--------------------------------------------------------------------------
| Theme Files
|--------------------------------------------------------------------------
|
| What a theme folder may hold: theme.json, layouts, the templates of
| the pages the help center renders, partials and assets of the kinds a
| browser loads. Anything else, a path that climbs out of the folder or
| a hidden file is a problem, so a theme never smuggles in what the
| server would not serve.
|
*/

package themes

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

/*
Pages returns the templates the help center renders, by name. A theme's
templates folder may hold these only.
*/
func Pages() []string {
	return []string{
		"home", "collection", "article", "search", "404",
		"changelog", "changelog_entry", "changelog_subscription",
		"feature_requests", "feature_request", "feature_request_new",
		"sign_in", "sign_in_code", "sign_in_return",
		"tickets", "ticket", "ticket_new",
		"demo_sign_in", "demo_conversations", "demo_conversation", "demo_conversation_new",
	}
}

/*
assetType names the content type of an asset a theme may serve, by its
extension, and false for a kind it may not.
*/
func assetType(name string) (string, bool) {
	ext := strings.ToLower(path.Ext(name))
	if kind, ok := mediaType(ext); ok {
		return kind, true
	}
	switch ext {
	case ".css":
		return "text/css; charset=utf-8", true
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8", true
	case ".json", ".map":
		return "application/json", true
	case ".txt":
		return "text/plain; charset=utf-8", true
	}
	return "", false
}

/*
mediaType names the content type of an image or font by its extension.
*/
func mediaType(ext string) (string, bool) {
	switch ext {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	case ".avif":
		return "image/avif", true
	case ".svg":
		return "image/svg+xml", true
	case ".ico":
		return "image/x-icon", true
	case ".woff":
		return "font/woff", true
	case ".woff2":
		return "font/woff2", true
	case ".ttf":
		return "font/ttf", true
	case ".otf":
		return "font/otf", true
	}
	return "", false
}

/*
ignored are files a theme folder may carry that the help center never
reads, such as its readme.
*/
func ignored(name string) bool {
	base := strings.ToUpper(path.Base(name))
	return !strings.Contains(name, "/") && (strings.HasPrefix(base, "README") || strings.HasPrefix(base, "LICENSE"))
}

/*
checkPath says what is wrong with a file's path or size, empty when
nothing is.
*/
func checkPath(name string, size int) string {
	clean := path.Clean(name)
	switch {
	case clean != name || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") ||
		strings.HasPrefix(name, "../"):
		return "is not a plain path inside the theme"
	case size > MaxFileBytes:
		return fmt.Sprintf("is larger than %d MB", MaxFileBytes>>20)
	case name == ManifestFile || ignored(name):
		return ""
	case hidden(name):
		return "hidden files are not part of a theme"
	case strings.HasPrefix(name, AssetDir):
		if _, ok := assetType(name); !ok {
			return "assets may be CSS, JavaScript, JSON, images or fonts"
		}
		return ""
	case !strings.HasSuffix(name, ".liquid"):
		return "templates, layouts and partials end in .liquid"
	case strings.HasPrefix(name, LayoutDir), strings.HasPrefix(name, PartialDir):
		return ""
	case strings.HasPrefix(name, TemplateDir):
		return checkPage(name)
	}
	return "a theme holds theme.json and the layouts, templates, partials and assets folders"
}

func hidden(name string) bool {
	for part := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func checkPage(name string) string {
	page := strings.TrimSuffix(strings.TrimPrefix(name, TemplateDir), ".liquid")
	if !slices.Contains(Pages(), page) {
		return "no page is called " + page + "; pages are " + strings.Join(Pages(), ", ")
	}
	return ""
}
