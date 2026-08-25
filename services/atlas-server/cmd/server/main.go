package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/server"
	"github.com/atlas-build/atlas-server/internal/store"
)

func main() {
	cfg := config.Load()

	userStore, err := store.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer userStore.Close()
	if err := userStore.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := bootstrapDefaultUser(cfg, userStore); err != nil {
		log.Fatalf("bootstrap user: %v", err)
	}

	authStore := store.NewAuthStore(userStore)
	pages, err := loadPages()
	if err != nil {
		log.Fatalf("parse web pages: %v", err)
	}

	handler := server.New(cfg, authStore, userStore, pages)
	log.Printf("atlas-server listening on %s (public base %s)", cfg.Addr, cfg.PublicBaseURL)
	log.Printf("account login: %s/login", cfg.PublicBaseURL)
	log.Printf("device login: %s/oauth2/device (requires web login + machine code)", cfg.PublicBaseURL)
	if err := http.ListenAndServe(cfg.Addr, handler); err != nil {
		log.Fatal(err)
	}
}

func bootstrapDefaultUser(cfg config.Config, users *store.MySQLStore) error {
	machineCode, err := store.GenerateMachineCode()
	if err != nil {
		return err
	}
	u := store.User{
		UserID:        cfg.DefaultUserID,
		Email:         cfg.DefaultEmail,
		FirstName:     "Atlas",
		LastName:      "Dev",
		PrincipalType: "User",
		PrincipalID:   cfg.DefaultUserID,
		MachineCode:   machineCode,
	}
	if err := users.EnsureBootstrapUser(u, cfg.BootstrapPassword); err != nil {
		return err
	}
	return users.SetMachineCodeIfEmpty(cfg.DefaultUserID, machineCode)
}

func loadPages() (server.Pages, error) {
	devicePath := envPagePath("ATLAS_DEVICE_PAGE", "web/device/index.html")
	loginPath := envPagePath("ATLAS_LOGIN_PAGE", "web/login/index.html")
	accountPath := envPagePath("ATLAS_ACCOUNT_PAGE", "web/account/index.html")

	device, err := template.ParseFiles(devicePath)
	if err != nil {
		return server.Pages{}, err
	}
	login, err := template.ParseFiles(loginPath)
	if err != nil {
		return server.Pages{}, err
	}
	account, err := template.ParseFiles(accountPath)
	if err != nil {
		return server.Pages{}, err
	}
	return server.Pages{Device: device, Login: login, Account: account}, nil
}

func envPagePath(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	for _, c := range pageCandidates(fallback) {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return fallback
}

func pageCandidates(rel string) []string {
	out := []string{rel, filepath.Join("services", "atlas-server", rel)}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		out = append(out,
			filepath.Join(dir, rel),
			filepath.Join(dir, "..", rel),
		)
	}
	return out
}
