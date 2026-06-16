package security

import (
	"log"
	"net/http"
	"strings"
)

var corsAllowHeaders = []string{
	"Authorization",
	"Content-Type",
	"Accept",
	"MCP-Protocol-Version",
	"Mcp-Session-Id",
	"Last-Event-ID",
}

func WrapOriginProtection(next http.Handler, allowedOrigins []string, requireOriginHeader bool, logger *log.Logger) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		value := strings.TrimSpace(origin)
		if value == "" {
			continue
		}
		allowed[value] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			if requireOriginHeader {
				http.Error(w, "Origin header is required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := allowed[origin]; !ok {
			if logger != nil {
				logger.Printf("reject origin=%q path=%s", origin, r.URL.Path)
			}
			http.Error(w, "Origin is not allowed", http.StatusForbidden)
			return
		}

		setCORSHeaders(w, origin)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	headers := w.Header()
	headers.Set("Access-Control-Allow-Origin", origin)
	headers.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	headers.Set("Access-Control-Allow-Headers", strings.Join(corsAllowHeaders, ", "))
	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Method")
	headers.Add("Vary", "Access-Control-Request-Headers")
}
