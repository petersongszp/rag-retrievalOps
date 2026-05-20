package router

import (
	interview "interview-agents/api/handler/interview"
	"interview-agents/internal/config"
	"log"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func RegisterCustomRoutes(r *server.Hertz) {
	asr := r.Group("/api/interview/asr")
	asr.GET("/capability", interview.GetASRCapability)
	asr.POST("/transcribe", interview.TranscribeInterviewAudio)

	prediction := r.Group("/api/prediction")
	prediction.POST("/delete", interview.DeletePredictionRecords)

	if config.Global.RAG.Enabled {
		registerKnowledgeBaseRoutes(r)
	} else {
		log.Println("[RAG:L0] rag.enabled=false, skip /api/kb route registration")
	}
}
