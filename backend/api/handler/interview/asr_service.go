package interview

import (
	"context"
	"errors"
	"io"
	"log"
	"mime/multipart"

	"interview-agents/api/response"
	"interview-agents/internal/middleware"
	asrservice "interview-agents/internal/service/interview/asr"

	"github.com/cloudwego/hertz/pkg/app"
)

const maxASRAudioFileSize = 50 * 1024 * 1024

var getASRService = asrservice.GetGlobalService

func GetASRCapability(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	capability, err := getASRService().GetCapability(ctx, userID)
	if err != nil {
		handleASRError(ctx, c, err)
		return
	}

	response.Success(ctx, c, capability)
}

func TranscribeInterviewAudio(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}
	sessionID := c.PostForm("session_id")
	interviewType := c.PostForm("interview_type")
	domain := c.PostForm("domain")

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		response.BadRequest(ctx, c, "file is required")
		return
	}

	audioBytes, err := readUploadedAudio(fileHeader)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	log.Printf(
		"[ASR] transcription request user_id=%d session_id=%s file_name=%s size=%d content_type=%s interview_type=%s domain=%s",
		userID,
		sessionID,
		fileHeader.Filename,
		len(audioBytes),
		fileHeader.Header.Get("Content-Type"),
		interviewType,
		domain,
	)

	result, err := getASRService().Transcribe(ctx, userID, asrservice.AudioTranscriptionRequest{
		FileName:      fileHeader.Filename,
		ContentType:   fileHeader.Header.Get("Content-Type"),
		AudioBytes:    audioBytes,
		SessionID:     sessionID,
		InterviewType: interviewType,
		Domain:        domain,
	})
	if err != nil {
		log.Printf(
			"[ASR] transcription failed user_id=%d session_id=%s err=%v",
			userID,
			sessionID,
			err,
		)
		handleASRError(ctx, c, err)
		return
	}

	log.Printf(
		"[ASR] transcription success user_id=%d session_id=%s provider=%s model=%s trace_id=%s text=%q",
		userID,
		sessionID,
		result.Provider,
		result.Model,
		result.TraceID,
		result.Text,
	)

	response.Success(ctx, c, result)
}

func readUploadedAudio(fileHeader *multipart.FileHeader) ([]byte, error) {
	if fileHeader.Size <= 0 {
		return nil, errors.New("uploaded audio file is empty")
	}
	if fileHeader.Size > maxASRAudioFileSize {
		return nil, errors.New("audio file size must not exceed 50MB")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer func(file multipart.File) {
		_ = file.Close()
	}(file)

	audioBytes, err := io.ReadAll(io.LimitReader(file, maxASRAudioFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(audioBytes) == 0 {
		return nil, errors.New("uploaded audio file is empty")
	}
	if len(audioBytes) > maxASRAudioFileSize {
		return nil, errors.New("audio file size must not exceed 50MB")
	}

	return audioBytes, nil
}

func handleASRError(ctx context.Context, c *app.RequestContext, err error) {
	var serviceErr *asrservice.ServiceError
	if !errors.As(err, &serviceErr) {
		response.InternalServerError(ctx, c, "ASR request failed")
		return
	}

	switch serviceErr.Code {
	case asrservice.ErrorCodeNotConfigured:
		response.ErrorWithData(ctx, c, serviceErr.StatusCode, serviceErr.Code, map[string]interface{}{
			"reason": serviceErr.Reason,
		})
	case asrservice.ErrorCodeRateLimitExceeded:
		response.ErrorWithData(ctx, c, serviceErr.StatusCode, serviceErr.Code, map[string]interface{}{
			"retry_after_seconds": serviceErr.RetryAfterSeconds,
		})
	case asrservice.ErrorCodeUnavailable:
		data := map[string]interface{}{}
		if serviceErr.TraceID != "" {
			data["trace_id"] = serviceErr.TraceID
		}
		response.ErrorWithData(ctx, c, serviceErr.StatusCode, serviceErr.Code, data)
	default:
		response.InternalServerError(ctx, c, serviceErr.Code)
	}
}
