package langserver

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func PathHasPrefix(s, prefix string) bool {
	var prefixSlash string
	if prefix != "" && !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefixSlash = prefix + string(os.PathSeparator)
	}
	return s == prefix || strings.HasPrefix(s, prefixSlash)
}

func PathTrimPrefix(s, prefix string) string {
	if s == prefix {
		return ""
	}
	if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefix += string(os.PathSeparator)
	}
	return strings.TrimPrefix(s, prefix)
}

func pathEqual(a, b string) bool {
	return PathTrimPrefix(a, b) == ""
}

// IsVendorDir tells if the specified directory is a vendor directory.
func IsVendorDir(dir string) bool {
	return strings.HasPrefix(dir, "vendor/") || strings.Contains(dir, "/vendor/")
}

// isURI tells if s denotes an URI
func isURI(s string) bool {
	return strings.HasPrefix(s, "file:///")
}

// pathToURI converts given absolute path to file URI
func pathToURI(path string) string {
	return "file://" + path
}

// uriToPath converts given file URI to path
func uriToPath(uri string) string {
	comps, _ := url.Parse(uri)
	path := comps.Path
	if runtime.GOOS == "windows" {
		// path would be something like "/d:/go/src/"
		// didOpen assume return path must start from "/", whereres hover assume return path is valid path.
		// So I decide to return correct path and modify didOpen
		// because this function name suggest this is what we should do.

		// remove root / and convert to backslash.
		return filepath.Clean(path[1:])
	}
	return path
}

// panicf takes the return value of recover() and outputs data to the log with
// the stack trace appended. Arguments are handled in the manner of
// fmt.Printf. Arguments should format to a string which identifies what the
// panic code was doing. Returns a non-nil error if it recovered from a panic.
func panicf(r interface{}, format string, v ...interface{}) error {
	if r != nil {
		// Same as net/http
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		id := fmt.Sprintf(format, v...)
		log.Printf("panic serving %s: %v\n%s", id, r, string(buf))
		return fmt.Errorf("unexpected panic: %v", r)
	}
	return nil
}
