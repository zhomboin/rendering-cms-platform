package config

import (
	"errors"
	"io"
	"os"
	"strings"
)

type PostBridgeConfig struct {
	Origin      string
	Environment string
	ServiceID   string
	SourceID    string
	Accounts    []string
	Scopes      []string
	ClientID    string
	KeyID       string
	Secret      string
}

func loadPostBridge() (PostBridgeConfig, error) {
	origin := strings.TrimSpace(os.Getenv("POSTBRIDGE_ORIGIN"))
	if origin == "" {
		return PostBridgeConfig{}, nil
	}
	cfg := PostBridgeConfig{
		Origin:      origin,
		Environment: strings.TrimSpace(os.Getenv("POSTBRIDGE_ENVIRONMENT")),
		ServiceID:   strings.TrimSpace(os.Getenv("POSTBRIDGE_SERVICE_ID")),
		SourceID:    strings.TrimSpace(os.Getenv("POSTBRIDGE_SOURCE_ID")),
		Accounts:    parseCSVEnvOrFallback("POSTBRIDGE_ACCOUNTS", ""),
		Scopes:      parseCSVEnvOrFallback("POSTBRIDGE_SCOPES", ""),
		ClientID:    strings.TrimSpace(os.Getenv("POSTBRIDGE_CLIENT_ID")),
		KeyID:       strings.TrimSpace(os.Getenv("POSTBRIDGE_KEY_ID")),
	}
	path := os.Getenv("POSTBRIDGE_CLIENT_SECRET_FILE")
	if path == "" {
		return cfg, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return PostBridgeConfig{}, errors.New("could not open POSTBRIDGE_CLIENT_SECRET_FILE")
	}
	defer file.Close()
	const maxSecretFileBytes = 128
	value, err := io.ReadAll(io.LimitReader(file, maxSecretFileBytes+1))
	if err != nil || len(value) > maxSecretFileBytes {
		return PostBridgeConfig{}, errors.New("could not read POSTBRIDGE_CLIENT_SECRET_FILE")
	}
	cfg.Secret = strings.TrimSpace(string(value))
	if cfg.Secret == "" {
		return PostBridgeConfig{}, errors.New("POSTBRIDGE_CLIENT_SECRET_FILE is empty")
	}
	return cfg, nil
}
