package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr      string
	EMQXBroker    string
	EMQXClientID  string
	EMQXUsername  string
	EMQXToken     string
	EMQXSubTopics []string
}

func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	broker := getenv("EMQX_BROKER", "tcp://emqx:1883")
	clientID := getenv("EMQX_CLIENT_ID", "beeminipos-backend")
	username := getenv("EMQX_USERNAME", "beeminipos-backend")
	token := os.Getenv("EMQX_TOKEN")
	subTopics := splitCSV(getenv("EMQX_SUB_TOPICS", "devices/+/telemetry,devices/+/status"))

	return Config{
		HTTPAddr:      addr,
		EMQXBroker:    broker,
		EMQXClientID:  clientID,
		EMQXUsername:  username,
		EMQXToken:     token,
		EMQXSubTopics: subTopics,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		item := strings.TrimSpace(p)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
