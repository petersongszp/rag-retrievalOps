package asr

import "context"

const (
	ProviderSiliconFlow           = "siliconflow"
	CapabilityReasonNotConfigured = "NOT_CONFIGURED"

	ErrorCodeNotConfigured     = "ASR_NOT_CONFIGURED"
	ErrorCodeUnavailable       = "ASR_UNAVAILABLE"
	ErrorCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
)

type Service interface {
	GetCapability(ctx context.Context, userID uint) (*Capability, error)
	Transcribe(ctx context.Context, userID uint, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error)
}

type Guard interface {
	CheckCapability(ctx context.Context) (*Capability, error)
	AllowUser(ctx context.Context, userID uint, model string) error
	AllowProvider(ctx context.Context, provider string, model string) error
}

type AudioTranscriptionProvider interface {
	Transcribe(ctx context.Context, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error)
}

type TranscriptModifier interface {
	Modify(ctx context.Context, req TranscriptModifyRequest) (string, error)
}

type Capability struct {
	Enabled  bool   `json:"enabled"`
	Reason   string `json:"reason,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type AudioTranscriptionRequest struct {
	FileName      string
	ContentType   string
	AudioBytes    []byte
	ModelName     string
	SessionID     string
	InterviewType string
	Domain        string
	QuestionText  string
}

type AudioTranscriptionResult struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	TraceID  string `json:"trace_id,omitempty"`
}

type TranscriptModifyRequest struct {
	QuestionText string
	Transcript   string
}

type ServiceError struct {
	Code              string
	StatusCode        int
	Reason            string
	RetryAfterSeconds int
	TraceID           string
	Err               error
}

func (e *ServiceError) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

func NewNotConfiguredError() *ServiceError {
	return &ServiceError{
		Code:       ErrorCodeNotConfigured,
		StatusCode: 503,
		Reason:     CapabilityReasonNotConfigured,
	}
}

func NewUnavailableError(traceID string, err error) *ServiceError {
	return &ServiceError{
		Code:       ErrorCodeUnavailable,
		StatusCode: 503,
		TraceID:    traceID,
		Err:        err,
	}
}

func NewRateLimitExceededError(retryAfterSeconds int, err error) *ServiceError {
	return &ServiceError{
		Code:              ErrorCodeRateLimitExceeded,
		StatusCode:        429,
		RetryAfterSeconds: retryAfterSeconds,
		Err:               err,
	}
}
