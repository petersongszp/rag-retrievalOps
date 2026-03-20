package asr

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type ASRConfig struct {
	BaseURL   string
	APIKey    string
	ModelName string
}

func LoadConfigFromEnv() ASRConfig {
	return ASRConfig{
		BaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_ASR_BASE_URL")), "/"),
		APIKey:    strings.TrimSpace(os.Getenv("OPENAI_ASR_API_KEY")),
		ModelName: strings.TrimSpace(os.Getenv("OPENAI_ASR_MODEL_NAME")),
	}
}

func (c ASRConfig) Validate() error {
	if c.BaseURL == "" || c.APIKey == "" || c.ModelName == "" {
		return errors.New("asr config is incomplete")
	}

	parsedURL, err := url.ParseRequestURI(c.BaseURL)
	if err != nil {
		return err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("asr base url must use http or https")
	}

	return nil
}

func (c ASRConfig) Capability() *Capability {
	if err := c.Validate(); err != nil {
		return &Capability{
			Enabled: false,
			Reason:  CapabilityReasonNotConfigured,
		}
	}

	return &Capability{
		Enabled:  true,
		Provider: ProviderSiliconFlow,
		Model:    c.ModelName,
	}
}
