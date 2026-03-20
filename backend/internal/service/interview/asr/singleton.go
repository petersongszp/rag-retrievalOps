package asr

import (
	"sync"

	"interview-agents/internal/repository"
)

var (
	globalService Service
	serviceOnce   sync.Once
)

func GetGlobalService() Service {
	serviceOnce.Do(func() {
		globalService = NewService(LoadConfigFromEnv(), repository.GetRedis())
	})

	return globalService
}

func ResetGlobalServiceForTesting() {
	globalService = nil
	serviceOnce = sync.Once{}
}
