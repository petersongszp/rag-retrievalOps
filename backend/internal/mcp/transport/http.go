package transport

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	internalmcp "interview-agents/internal/mcp"
	"interview-agents/internal/mcp/security"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RunHTTP(ctx context.Context, cfg internalmcp.Config, server *mcp.Server, logger *log.Logger) error {
	if server == nil {
		return errors.New("server is required")
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           NewHTTPHandler(cfg, server, logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		if logger != nil {
			logger.Printf("starting http transport addr=%s endpoint=%s", cfg.ListenAddress(), cfg.Endpoint)
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func NewHTTPHandler(cfg internalmcp.Config, server *mcp.Server, logger *log.Logger) http.Handler {
	mux := http.NewServeMux()
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: cfg.SessionTimeout,
		Stateless:      true,
	})

	var endpointHandler http.Handler = mcpHandler
	if !cfg.DisableHTTPAuth {
		endpointHandler = mcpauth.RequireBearerToken(passThroughTokenVerifier(), nil)(endpointHandler)
	}
	endpointHandler = security.WrapOriginProtection(endpointHandler, cfg.AllowedOrigins, cfg.RequireOriginHeader, logger)
	endpointHandler = withRequestLogging(endpointHandler, logger)

	mux.Handle(cfg.Endpoint, endpointHandler)
	mux.HandleFunc("/healthz", healthzHandler)
	mux.Handle("/readyz", newReadyzHandler(cfg))
	if !cfg.DisableMetrics {
		mux.Handle("/metrics", promhttp.Handler())
	}
	return mux
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func newReadyzHandler(cfg internalmcp.Config) http.Handler {
	client := &http.Client{Timeout: minDuration(cfg.Timeout, 3*time.Second)}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, cfg.UpstreamReadyURL(), nil)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "message": err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status":          "degraded",
				"upstream_status": resp.StatusCode,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func passThroughTokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		if token == "" {
			return nil, mcpauth.ErrInvalidToken
		}
		return &mcpauth.TokenInfo{
			Expiration: time.Now().Add(24 * time.Hour),
			UserID:     security.AuthorizationFingerprint(req.Header.Get("Authorization")),
		}, nil
	}
}

func withRequestLogging(next http.Handler, logger *log.Logger) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Printf(
			"http request method=%s path=%s status=%d duration_ms=%d origin=%q auth=%s",
			r.Method,
			r.URL.Path,
			recorder.statusCode,
			time.Since(startedAt).Milliseconds(),
			r.Header.Get("Origin"),
			security.AuthorizationFingerprint(r.Header.Get("Authorization")),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if a < b {
		return a
	}
	return b
}
