package main

import (
	"strings"
	"testing"
)

func TestResolveAgentIdentityPrefersQuery(t *testing.T) {
	got := resolveAgentIdentity("  laptop-a ", "user1")
	if got != "laptop-a" {
		t.Fatalf("got %q, want laptop-a", got)
	}
}

func TestResolveAgentIdentityFallsBackToUser(t *testing.T) {
	got := resolveAgentIdentity("", "user1")
	if got != "user1" {
		t.Fatalf("got %q, want user1", got)
	}
}

func TestResolveAgentIdentityAnonymousWhenEmpty(t *testing.T) {
	got := resolveAgentIdentity("  ", "")
	if !strings.HasPrefix(got, "anonymous-") {
		t.Fatalf("got %q, want anonymous-*", got)
	}
}
