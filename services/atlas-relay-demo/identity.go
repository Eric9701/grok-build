package main

import (
	"fmt"
	"strings"
	"sync/atomic"
)

var anonSeq atomic.Uint64

func resolveAgentIdentity(queryID, userID string) string {
	if id := cleanID(queryID); id != "" {
		return id
	}
	if id := cleanID(userID); id != "" {
		return id
	}
	return fmt.Sprintf("anonymous-%d", anonSeq.Add(1))
}

func cleanID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}
