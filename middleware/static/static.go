package static

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Config defines static file server configuration
type Config struct {
	// Root directory to serve files from
	Root string
	
	// Index file names
	Index []string
	
	// Browse enables directory browsing
	Browse bool
	
	// HTML5 enables HTML5 mode (fallback to index.html)
	HTML5 bool
	
	// Skipper defines a function to skip static file serving
	Skipper middleware.Skipper
	
	// IgnoreBase ignores base path from URL
	IgnoreBase bool
	
	// EnableCache enables cache headers
	EnableCache bool
	
	// CacheMaxAge sets max-age for cache control (in seconds)
	CacheMaxAge int
	
	// EnableETag enables ETag generation
	EnableETag bool
	
	// EnableGzip enables pre-compressed .gz file serving
	EnableGzip bool
	
	// EnableBrotli enables pre-compressed .br file serving
	EnableBrotli bool
}

// DefaultConfig returns default static file configuration
func DefaultConfig() Config {
	return Config{
		Root:         "public",
		Index:        []string{"index.html"},
		Browse:       false,
		HTML5:        false,
		Skipper:      middleware.DefaultSkipper,
		IgnoreBase:   false,
		EnableCache:  true,
		CacheMaxAge:  3600, // 1 hour
		EnableETag:   true,
		EnableGzip:   true,
		EnableBrotli: true,
	}
}

// Static returns a static file server middleware
func Static(root string) echo.MiddlewareFunc {
	config := DefaultConfig()
	config.Root = root
	return StaticWithConfig(config)
}

// StaticWithConfig returns a static file server middleware with config
func StaticWithConfig(config Config) echo.MiddlewareFunc {
	// Set defaults
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}
	
	if len(config.Index) == 0 {
		config.Index = DefaultConfig().Index
	}
	
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}
			
			p := c.Request().URL.Path
			if config.IgnoreBase {
				p = c.Param("*")
			}
			
			// Clean path but preserve trailing slash for directories
			trailingSlash := strings.HasSuffix(p, "/")
			p = filepath.Clean(p)
			if p == "." {
				p = "/"
			} else if trailingSlash && p != "/" {
				p += "/"
			}
			
			// Check for pre-compressed files
			if config.EnableBrotli && strings.Contains(c.Request().Header.Get("Accept-Encoding"), "br") {
				if err := serveFile(c, config, filepath.Join(config.Root, p+".br"), p, "br"); err == nil {
					return nil
				}
			}
			
			if config.EnableGzip && strings.Contains(c.Request().Header.Get("Accept-Encoding"), "gzip") {
				if err := serveFile(c, config, filepath.Join(config.Root, p+".gz"), p, "gzip"); err == nil {
					return nil
				}
			}
			
			// Serve regular file
			file := filepath.Join(config.Root, p)
			fi, err := os.Stat(file)
			
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				
				// Try index files
				if p[len(p)-1] == '/' {
					for _, index := range config.Index {
						indexPath := filepath.Join(file, index)
						if fi, err = os.Stat(indexPath); err == nil {
							file = indexPath
							break
						}
					}
				}
				
				if err != nil {
					// HTML5 mode fallback
					if config.HTML5 && filepath.Ext(p) == "" {
						for _, index := range config.Index {
							indexPath := filepath.Join(config.Root, index)
							if _, err := os.Stat(indexPath); err == nil {
								return serveFile(c, config, indexPath, index, "")
							}
						}
					}
					
					return next(c)
				}
			}
			
			// Directory handling
			if fi.IsDir() {
				// Redirect to trailing slash
				if p[len(p)-1] != '/' {
					return c.Redirect(http.StatusMovedPermanently, p+"/")
				}
				
				// Try index files
				for _, index := range config.Index {
					indexPath := filepath.Join(file, index)
					if fi, err := os.Stat(indexPath); err == nil && !fi.IsDir() {
						return serveFile(c, config, indexPath, filepath.Join(p, index), "")
					}
				}
				
				// Directory browsing
				if config.Browse {
					return listDirectory(c, file, p)
				}
				
				return echo.ErrNotFound
			}
			
			return serveFile(c, config, file, p, "")
		}
	}
}

func serveFile(c echo.Context, config Config, file, name, encoding string) error {
	fi, err := os.Stat(file)
	if err != nil {
		return err
	}
	
	// Set content encoding for pre-compressed files
	if encoding != "" {
		c.Response().Header().Set("Content-Encoding", encoding)
		c.Response().Header().Set("Vary", "Accept-Encoding")
	}
	
	// Generate ETag
	if config.EnableETag {
		etag := generateETag(file, fi)
		c.Response().Header().Set("ETag", etag)
		
		// Check If-None-Match
		if match := c.Request().Header.Get("If-None-Match"); match != "" {
			if match == etag {
				return c.NoContent(http.StatusNotModified)
			}
		}
	}
	
	// Set cache headers
	if config.EnableCache {
		c.Response().Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", config.CacheMaxAge))
	} else {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")
	}
	
	// Set Last-Modified
	c.Response().Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
	
	// Check If-Modified-Since
	if config.EnableCache {
		if t, err := time.Parse(http.TimeFormat, c.Request().Header.Get("If-Modified-Since")); err == nil {
			if fi.ModTime().Unix() <= t.Unix() {
				return c.NoContent(http.StatusNotModified)
			}
		}
	}
	
	// Handle range requests
	if c.Request().Header.Get("Range") != "" {
		return serveFileWithRange(c, file, name, fi)
	}
	
	// For pre-compressed files, we need to serve the file manually to preserve content type
	if encoding != "" {
		// Set content type based on original file name
		if ext := filepath.Ext(name); ext != "" {
			if ct := getContentType(ext); ct != "" {
				c.Response().Header().Set("Content-Type", ct)
			}
		}
		
		// Open and serve the file
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		
		return c.Stream(http.StatusOK, c.Response().Header().Get("Content-Type"), f)
	}
	
	return c.File(file)
}

func generateETag(file string, fi os.FileInfo) string {
	// Simple ETag based on file path, size, and modification time
	h := md5.New()
	h.Write([]byte(file))
	h.Write([]byte(strconv.FormatInt(fi.Size(), 36)))
	h.Write([]byte(strconv.FormatInt(fi.ModTime().Unix(), 36)))
	return fmt.Sprintf(`"%x"`, h.Sum(nil))
}

func serveFileWithRange(c echo.Context, file, name string, fi os.FileInfo) error {
	// Parse range header
	rangeHeader := c.Request().Header.Get("Range")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return c.File(file)
	}
	
	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return c.File(file)
	}
	
	var start, end int64
	var err error
	
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return c.File(file)
		}
	} else if parts[1] != "" {
		// Suffix range like "-10" means last 10 bytes
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return c.File(file)
		}
		start = fi.Size() - end
		end = fi.Size() - 1
		if start < 0 {
			start = 0
		}
		goto serveRange
	}
	
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return c.File(file)
		}
	} else {
		end = fi.Size() - 1
	}
	
serveRange:
	if start > end || end >= fi.Size() {
		c.Response().Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fi.Size()))
		return c.NoContent(http.StatusRequestedRangeNotSatisfiable)
	}
	
	// Open file
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Seek to start position
	_, err = f.Seek(start, 0)
	if err != nil {
		return err
	}
	
	// Set headers
	c.Response().Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fi.Size()))
	c.Response().Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	c.Response().Header().Set("Accept-Ranges", "bytes")
	c.Response().WriteHeader(http.StatusPartialContent)
	
	// Copy the requested range
	_, err = io.CopyN(c.Response(), f, end-start+1)
	return err
}

func listDirectory(c echo.Context, dir, path string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	
	// Simple HTML directory listing
	var html strings.Builder
	html.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Index of ` + path + `</title>
    <style>
        body { font-family: monospace; margin: 20px; }
        h1 { font-size: 1.5em; }
        table { border-collapse: collapse; }
        td { padding: 5px 20px 5px 0; }
        a { text-decoration: none; color: #0066cc; }
        a:hover { text-decoration: underline; }
        .size { text-align: right; }
        .date { color: #666; }
    </style>
</head>
<body>
    <h1>Index of ` + path + `</h1>
    <table>
        <tr><th>Name</th><th>Size</th><th>Modified</th></tr>`)
	
	// Parent directory link
	if path != "/" {
		html.WriteString(`<tr><td><a href="../">../</a></td><td></td><td></td></tr>`)
	}
	
	// List files and directories
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		
		name := f.Name()
		if f.IsDir() {
			name += "/"
		}
		
		size := ""
		if !f.IsDir() {
			size = formatSize(info.Size())
		}
		
		html.WriteString(fmt.Sprintf(
			`<tr><td><a href="%s">%s</a></td><td class="size">%s</td><td class="date">%s</td></tr>`,
			name, name, size, info.ModTime().Format("2006-01-02 15:04:05"),
		))
	}
	
	html.WriteString(`
    </table>
</body>
</html>`)
	
	return c.HTML(http.StatusOK, html.String())
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func getContentType(ext string) string {
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return ""
	}
}