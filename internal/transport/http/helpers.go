package http

import (
	"crypto/rand"
	"fmt"
)

// generateSessionID returns a UUID-format random session ID using crypto/rand.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generateSessionID: crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
