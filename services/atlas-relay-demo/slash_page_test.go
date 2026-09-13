package main

import (
	"os"
	"strings"
	"testing"
)

func TestChatPageWiresSlashCatalog(t *testing.T) {
	raw, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, needle := range []string{
		`src="/md.js"`,
		`src="/slash.js"`,
		`available_commands_update`,
		`x.ai/commands/list`,
		`id="cmdPanel"`,
		`id="slashMenu"`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("index.html missing %q", needle)
		}
	}
	if !strings.Contains(string(slashJS), "AtlasSlash") {
		t.Fatal("embedded slash.js missing AtlasSlash")
	}
	if !strings.Contains(string(mdJS), "AtlasMD") {
		t.Fatal("embedded md.js missing AtlasMD")
	}
}
