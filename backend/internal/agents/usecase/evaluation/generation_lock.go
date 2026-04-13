package evaluation

import (
	"fmt"
	"sync"
)

var generationLocks sync.Map

type keyedLock struct {
	mu sync.Mutex
}

func acquireGenerationLock(kind string, userID uint, reportID uint64) func() {
	key := fmt.Sprintf("%s:%d:%d", kind, userID, reportID)
	value, _ := generationLocks.LoadOrStore(key, &keyedLock{})
	lk := value.(*keyedLock)
	lk.mu.Lock()
	return lk.mu.Unlock
}
