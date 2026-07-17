package httpserver

import (
	"context"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/settings"
)

const dynamicConcurrencyConfigCacheTTL = 15 * time.Second

func (r *Router) dynamicConcurrencyConfig(ctx context.Context) apiaccess.DynamicConcurrencyConfig {
	r.dynamicConcurrencyMu.RLock()
	if !r.dynamicConcurrencyCacheAt.IsZero() && time.Since(r.dynamicConcurrencyCacheAt) < dynamicConcurrencyConfigCacheTTL {
		config := r.dynamicConcurrencyCache
		r.dynamicConcurrencyMu.RUnlock()
		return config
	}
	r.dynamicConcurrencyMu.RUnlock()

	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("dynamic concurrency settings lookup failed", "error", err)
		}
		config := apiaccess.DefaultDynamicConcurrencyConfig()
		r.cacheDynamicConcurrencyConfig(config)
		return config
	}
	config := dynamicConcurrencyConfigFromSettings(values)
	r.cacheDynamicConcurrencyConfig(config)
	return config
}

func dynamicConcurrencyConfigFromSettings(values settings.Settings) apiaccess.DynamicConcurrencyConfig {
	return apiaccess.NormalizeDynamicConcurrencyConfig(apiaccess.DynamicConcurrencyConfig{
		Enabled:     anyBool(values["dynamicConcurrencyEnabled"]),
		WindowValue: anyInt(values["dynamicConcurrencyWindowValue"], 1),
		WindowUnit:  strings.TrimSpace(anyString(values["dynamicConcurrencyWindowUnit"])),
		RequestStep: anyInt(values["dynamicConcurrencyRequestStep"], 50),
		Increment:   anyInt(values["dynamicConcurrencyIncrement"], 5),
	})
}

func (r *Router) cacheDynamicConcurrencyConfig(config apiaccess.DynamicConcurrencyConfig) {
	r.dynamicConcurrencyMu.Lock()
	r.dynamicConcurrencyCache = apiaccess.NormalizeDynamicConcurrencyConfig(config)
	r.dynamicConcurrencyCacheAt = time.Now()
	r.dynamicConcurrencyMu.Unlock()
}
