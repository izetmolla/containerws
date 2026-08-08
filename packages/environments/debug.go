package environments

import (
	"fmt"
	"log"
)

func (c *Config) DebugEnabled() bool {
	return c != nil && c.debug
}

func (m *Environments) debug(step, format string, args ...any) {
	if m == nil || m.config == nil || !m.config.DebugEnabled() {
		return
	}
	log.Printf("environments[debug] %s: %s", step, fmt.Sprintf(format, args...))
}

func logEnvWarn(format string, args ...any) {
	log.Printf("environments: "+format, args...)
}

func truncHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8]
}
