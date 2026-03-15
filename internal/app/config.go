package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	CTEFile       string
	ContractsFile string
	DocsDir       string
}

func LoadConfig() Config {
	cwd, _ := os.Getwd()
	return Config{
		DatabaseURL:   getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tenderhack?sslmode=disable"),
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		CTEFile:       getenv("CTE_FILE", autodetectExcel(cwd, "СТЕ")),
		ContractsFile: getenv("CONTRACTS_FILE", autodetectExcel(cwd, "Контракты")),
		DocsDir:       getenv("DOCS_DIR", filepath.Join(cwd, "generated")),
	}
}

func ErrUnknownCommand(cmd string) error {
	return fmt.Errorf("unknown command %q; use serve, import, init-db", cmd)
}

func (c Config) ValidateImportFiles() error {
	if c.CTEFile == "" {
		return errors.New("CTE_FILE is not set and TenderHack_СТЕ_*.xlsx was not found")
	}
	if c.ContractsFile == "" {
		return errors.New("CONTRACTS_FILE is not set and TenderHack_Контракты_*.xlsx was not found")
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func autodetectExcel(root, contains string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".xlsx") || strings.HasPrefix(name, "~$") {
			continue
		}
		if strings.Contains(name, contains) {
			return filepath.Join(root, name)
		}
	}
	return ""
}
