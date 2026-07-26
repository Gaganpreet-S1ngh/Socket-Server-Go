package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// Recover catches panics in any handler so one bad connection/bug can't
// crash the entire server process. In production this is non-negotiable -
// without it, a single nil-pointer deref in a handler takes down every
// active websocket connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("Panic : %w\npath : %s\nremote_addr : %s\n RECOVERED FROM PANIC", err, r.URL.Path, r.RemoteAddr)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Hijack delegates to the underlying ResponseWriter's Hijacker.
// This is required for websocket upgrades to work through this
// middleware - without it, gorilla/websocket's Upgrade() fails with
// "response does not implement http.Hijacker" because our wrapper
// type doesn't expose the underlying connection by default.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}
