package plugin

import (
	"fmt"
	"strings"
	"sync"
)

var (
	runtimesMu sync.RWMutex
	runtimes   []Runtime
)

// RegisterRuntime registers a runtime for a plugin execution engine.
// Called from init() in each runtime implementation file.
// The prefix must be unique across all registered runtimes.
func RegisterRuntime(rt Runtime) {
	runtimesMu.Lock()
	defer runtimesMu.Unlock()

	prefix := rt.Prefix()
	for _, existing := range runtimes {
		if existing.Prefix() == prefix {
			panic(fmt.Sprintf("plugin: runtime with prefix %q already registered", prefix))
		}
	}
	runtimes = append(runtimes, rt)
}

// GetRuntimeForEntrypoint finds the first Runtime whose Prefix matches the
// given entrypoint string. Matching is prefix-based: an entrypoint like
// "ts:scripts/handler.ts" matches a runtime with Prefix "ts:".
func GetRuntimeForEntrypoint(entrypoint string) (Runtime, bool) {
	runtimesMu.RLock()
	defer runtimesMu.RUnlock()

	for _, rt := range runtimes {
		if strings.HasPrefix(entrypoint, rt.Prefix()) {
			return rt, true
		}
	}
	return nil, false
}

// RegisteredRuntimes returns all registered runtime prefixes (for diagnostics).
func RegisteredRuntimes() []string {
	runtimesMu.RLock()
	defer runtimesMu.RUnlock()

	prefixes := make([]string, len(runtimes))
	for i, rt := range runtimes {
		prefixes[i] = rt.Prefix()
	}
	return prefixes
}
