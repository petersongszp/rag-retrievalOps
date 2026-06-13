package parserprovider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"interview-agents/internal/documentparser"
)

const defaultMaxUploadBytes = 50 << 20

func NewParseHandler(parser Parser) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if parser == nil {
			writeProviderError(w, http.StatusInternalServerError, &documentparser.ProviderError{
				Code:      "parser_unavailable",
				Message:   "parser is not configured",
				Stage:     "parse",
				Retryable: true,
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, defaultMaxUploadBytes)
		if err := r.ParseMultipartForm(defaultMaxUploadBytes); err != nil {
			writeProviderError(w, http.StatusBadRequest, &documentparser.ProviderError{
				Code:      "invalid_multipart",
				Message:   fmt.Sprintf("parse multipart form: %v", err),
				Stage:     "parse",
				Retryable: false,
			})
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeProviderError(w, http.StatusBadRequest, &documentparser.ProviderError{
				Code:      "missing_file",
				Message:   "multipart field file is required",
				Stage:     "parse",
				Retryable: false,
			})
			return
		}
		defer func() {
			_ = file.Close()
		}()

		content, err := io.ReadAll(file)
		if err != nil {
			writeProviderError(w, http.StatusBadRequest, &documentparser.ProviderError{
				Code:      "read_file_failed",
				Message:   fmt.Sprintf("read uploaded file: %v", err),
				Stage:     "parse",
				Retryable: false,
			})
			return
		}
		fileName := header.Filename
		fileType := documentparser.NormalizeFileType(r.FormValue("file_type"))
		if fileType == "" {
			fileType = documentparser.NormalizeFileType(filepath.Ext(fileName))
		}

		doc, err := parser.Parse(r.Context(), ParseRequest{
			FileName: fileName,
			FileType: fileType,
			Content:  content,
		})
		if err != nil {
			if providerErr, ok := err.(*documentparser.ProviderError); ok {
				status := http.StatusUnprocessableEntity
				if providerErr.Retryable {
					status = http.StatusBadGateway
				}
				writeProviderError(w, status, providerErr)
				return
			}
			writeProviderError(w, http.StatusBadGateway, &documentparser.ProviderError{
				Code:      "parse_failed",
				Message:   err.Error(),
				Stage:     "parse",
				Retryable: true,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
}

func writeProviderError(w http.ResponseWriter, status int, providerErr *documentparser.ProviderError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(providerErr)
}
