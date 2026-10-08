package main

import (
	"os"
	"regexp"
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
		`src="/chat.js"`,
		`id="cmdPanel"`,
		`id="slashMenu"`,
		`id="toMobile"`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("index.html missing %q", needle)
		}
	}
	chat, err := os.ReadFile("web/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		`available_commands_update`,
		`x.ai/commands/list`,
		`atlas.relay/bind`,
	} {
		if !strings.Contains(string(chat), needle) {
			t.Fatalf("chat.js missing %q", needle)
		}
	}
	if string(chatJS) != string(chat) {
		t.Fatal("embedded chat.js does not match web/chat.js")
	}
	if !strings.Contains(string(slashJS), "AtlasSlash") {
		t.Fatal("embedded slash.js missing AtlasSlash")
	}
	if !strings.Contains(string(mdJS), "AtlasMD") {
		t.Fatal("embedded md.js missing AtlasMD")
	}
}

func TestMobilePageSharesChatHooks(t *testing.T) {
	page, err := os.ReadFile("web/m.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	if string(mobileHTML) != html {
		t.Fatal("embedded m.html does not match web/m.html")
	}
	for _, needle := range []string{
		`src="/md.js"`,
		`src="/slash.js"`,
		`src="/chat.js"`,
		`name="viewport"`,
		`viewport-fit=cover`,
		`id="toDesk"`,
		`/build`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("m.html missing %q", needle)
		}
	}
	desk, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	chat, err := os.ReadFile("web/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	ids := regexp.MustCompile(`getElementById\("([^"]+)"\)`).FindAllStringSubmatch(string(chat), -1)
	if len(ids) == 0 {
		t.Fatal("chat.js has no getElementById hooks")
	}
	seen := map[string]bool{}
	for _, m := range ids {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		needle := `id="` + id + `"`
		if !strings.Contains(html, needle) {
			t.Errorf("m.html missing hook %s", needle)
		}
		if !strings.Contains(string(desk), needle) {
			t.Errorf("index.html missing hook %s", needle)
		}
	}
}
