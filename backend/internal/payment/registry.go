package payment

import (
	"fmt"
	"sync"
)

// Registry 支付渠道注册中心
type Registry struct {
	mu        sync.RWMutex
	providers map[ProviderName]Provider
}

var globalRegistry = &Registry{
	providers: make(map[ProviderName]Provider),
}

// RegisterProvider 注册支付渠道
func RegisterProvider(p Provider) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.providers[p.Name()] = p
}

// GetProvider 获取支付渠道
func GetProvider(name ProviderName) (Provider, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	p, ok := globalRegistry.providers[name]
	if !ok {
		return nil, fmt.Errorf("payment provider %q not registered", name)
	}
	return p, nil
}

// GetProviderByString 根据字符串获取支付渠道
func GetProviderByString(name string) (Provider, error) {
	return GetProvider(ProviderName(name))
}
