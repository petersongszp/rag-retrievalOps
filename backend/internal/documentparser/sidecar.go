package documentparser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type ErrorSidecar struct {
	ErrorCode string       `json:"error_code"`
	Message   string       `json:"message"`
	Stage     string       `json:"stage,omitempty"`
	Page      int          `json:"page,omitempty"`
	Quality   ParseQuality `json:"quality,omitempty"`
}

func SaveNormalizedSidecar(ctx context.Context, sourcePath string, doc *NormalizedDocument) (string, error) {
	_ = ctx
	if err := doc.Validate(); err != nil {
		return "", err
	}
	path := sourcePath + ".normalized.json"
	if err := writeJSONFile(path, doc); err != nil {
		return "", fmt.Errorf("write normalized sidecar: %w", err)
	}
	return path, nil
}

func SaveErrorSidecar(ctx context.Context, sourcePath string, sidecar ErrorSidecar) (string, error) {
	_ = ctx
	path := sourcePath + ".normalized.error.json"
	if sidecar.ErrorCode == "" {
		sidecar.ErrorCode = "parse_error"
	}
	if err := writeJSONFile(path, sidecar); err != nil {
		return "", fmt.Errorf("write normalized error sidecar: %w", err)
	}
	return path, nil
}

func writeJSONFile(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
