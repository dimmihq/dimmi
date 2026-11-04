/*
|--------------------------------------------------------------------------
| Upload File Rules
|--------------------------------------------------------------------------
|
| A customer's upload ends up opened by a teammate, so files that run code
| or render as a page never get in. The name and declared type are checked
| when the upload starts, and the bytes themselves once they arrive, since
| a renamed program still starts with a program's header.
|
*/

package attachments

import (
	"bytes"
	"path"
	"strings"
)

/*
blockedExtension reports whether a file name runs code or renders as a page
when opened.
*/
func blockedExtension(filename string) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".exe", ".bat", ".cmd", ".com", ".scr", ".pif", ".vbs", ".js", ".msi", ".dmg", ".apk", ".jar", ".ps1",
		".sh", ".app", ".html", ".htm", ".xhtml":
		return true
	}

	return false
}

/*
blockedType reports whether a media type runs code or renders as a page.
*/
func blockedType(contentType string) bool {
	media, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(contentType)), ";")

	switch strings.TrimSpace(media) {
	case "application/x-msdownload", "application/x-executable", "application/x-mach-binary", "application/x-sh",
		"application/x-elf", "application/vnd.microsoft.portable-executable", "application/x-dosexec",
		"application/javascript", "text/javascript", "text/html", "application/xhtml+xml":
		return true
	}

	return false
}

/*
executableHeader reports whether the first bytes are a program's: Windows,
Linux and macOS binaries, and scripts that name their interpreter.
*/
func executableHeader(head []byte) bool {
	magics := [][]byte{
		[]byte("MZ"), []byte("\x7fELF"), []byte("#!"),
		{0xfe, 0xed, 0xfa, 0xce},
		{0xfe, 0xed, 0xfa, 0xcf},
		{0xce, 0xfa, 0xed, 0xfe},
		{0xcf, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe},
	}

	for _, magic := range magics {
		if bytes.HasPrefix(head, magic) {
			return true
		}
	}

	return false
}
