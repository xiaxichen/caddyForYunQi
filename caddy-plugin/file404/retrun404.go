package file404

import (
	"fmt"
	"github.com/caddyserver/caddy"
	"github.com/caddyserver/caddy/caddyhttp/httpserver"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	caddy.RegisterPlugin("file404", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

type handler404default struct {
	next             httpserver.Handler
	Log              *httpserver.Logger
	GenericErrorPage string         // default error page filename
	ErrorPages       map[int]string // map of status code to filename
}

func (g handler404default) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	defer g.recovery(w, r)

	status, err := g.next.ServeHTTP(w, r)

	if err != nil {
		errMsg := fmt.Sprintf("[ERROR %d %s] %v", status, r.URL.Path, err)

		g.Log.Println(errMsg)
	}
	if status >= 400 {
		g.errorPage(w, r, status)
		return 0, err
	}


	return status,err
}

// errorPage serves a static error page to w according to the status
// code. If there is an error serving the error page, a plaintext error
// message is written instead, and the extra error is logged.
func (h handler404default) errorPage(w http.ResponseWriter, r *http.Request, code int) {
	// See if an error page for this status code was specified
	if pagePath, ok := h.findErrorPage(code); ok {
		// Try to open it
		errorPage, err := os.Open(pagePath)
		if err != nil {
			// An additional error handling an error... <insert grumpy cat here>
			h.Log.Printf("[NOTICE %d %s] could not load error page: %v", code, r.URL.String(), err)
			httpserver.DefaultErrorFunc(w, r, code)
			return
		}
		defer errorPage.Close()
		// Get content type by extension
		contentType := mime.TypeByExtension(filepath.Ext(pagePath))
		if contentType == "" {
			contentType = "text/html; charset=utf-8"
		}
		// Copy the page body into the response
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(code)
		_, err = io.Copy(w, errorPage)

		if err != nil {
			// Epic fail... sigh.
			h.Log.Printf("[NOTICE %d %s] could not respond with %s: %v", code, r.URL.String(), pagePath, err)
			httpserver.DefaultErrorFunc(w, r, code)
		}

		return
	}

	// Default error response
	httpserver.DefaultErrorFunc(w, r, code)
}

func (h handler404default) findErrorPage(code int) (string, bool) {
	if pagePath, ok := h.ErrorPages[code]; ok {
		return pagePath, true
	}

	if h.GenericErrorPage != "" {
		return h.GenericErrorPage, true
	}

	return "", false
}

func (h handler404default) recovery(w http.ResponseWriter, r *http.Request) {
	rec := recover()
	if rec == nil {
		return
	}

	// Obtain source of panic
	// From: https://gist.github.com/swdunlop/9629168
	var name, file string // function name, file name
	var line int
	var pc [16]uintptr
	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	// Trim file path
	delim := "/github.com/caddyserver/caddy/"
	pkgPathPos := strings.Index(file, delim)
	if pkgPathPos > -1 && len(file) > pkgPathPos+len(delim) {
		file = file[pkgPathPos+len(delim):]
	}

	panicMsg := fmt.Sprintf("[PANIC %s] %s:%d - %v", r.URL.String(), file, line, rec)

	// Currently we don't use the function name, since file:line is more conventional
	h.Log.Printf(panicMsg)
	h.errorPage(w, r, http.StatusInternalServerError)

}