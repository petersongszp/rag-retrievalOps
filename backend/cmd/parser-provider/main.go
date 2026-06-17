package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"interview-agents/internal/parserprovider"
)

func main() {
	host := getenv("PARSER_PROVIDER_HOST", "0.0.0.0")
	port := getenv("PARSER_PROVIDER_PORT", "9000")
	doclingBaseURL := getenv("DOCLING_BASE_URL", "http://localhost:5001")
	doclingPath := getenv("DOCLING_CONVERT_PATH", "/v1/convert/file")
	timeout := time.Duration(getenvInt("DOCLING_TIMEOUT_MS", 120000)) * time.Millisecond

	docling := parserprovider.NewDoclingClient(parserprovider.DoclingConfig{
		BaseURL: doclingBaseURL,
		Path:    doclingPath,
		Timeout: timeout,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/parse", parserprovider.NewParseHandler(docling))

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", host, port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       timeout + 10*time.Second,
		WriteTimeout:      timeout + 10*time.Second,
	}

	go func() {
		log.Printf("[parser-provider] listening on %s, docling=%s%s", server.Addr, doclingBaseURL, doclingPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[parser-provider] server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[parser-provider] shutdown failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
