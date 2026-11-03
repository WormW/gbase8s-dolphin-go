package gbase

import (
	"testing"
	"time"
)

func TestNewDriverConfigUsesDolphinCompatibleDefaults(t *testing.T) {
	config, err := newDriverConfig(Config{
		Host:     "127.0.0.1",
		Database: "app",
		Username: "app_user",
		Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Addr != "127.0.0.1:3308" {
		t.Fatalf("Addr = %q, want 127.0.0.1:3308", config.Addr)
	}
	if !config.InterpolateParams {
		t.Fatal("InterpolateParams is false, want true")
	}
	if !config.ParseTime {
		t.Fatal("ParseTime is false, want true")
	}
	if config.Params["charset"] != "utf8mb4" {
		t.Fatalf("charset = %q, want utf8mb4", config.Params["charset"])
	}
	if config.Timeout != 5*time.Second || config.ReadTimeout != 30*time.Second || config.WriteTimeout != 30*time.Second {
		t.Fatalf("unexpected timeout defaults: %v, %v, %v", config.Timeout, config.ReadTimeout, config.WriteTimeout)
	}
}

func TestNewDriverConfigSupportsIPv6(t *testing.T) {
	config, err := newDriverConfig(Config{Host: "::1", Port: 43308, Database: "app", Username: "app_user"})
	if err != nil {
		t.Fatal(err)
	}
	if config.Addr != "[::1]:43308" {
		t.Fatalf("Addr = %q, want [::1]:43308", config.Addr)
	}
}

func TestNewDriverConfigRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "missing host", config: Config{Database: "app", Username: "app_user"}},
		{name: "missing database", config: Config{Host: "127.0.0.1", Username: "app_user"}},
		{name: "missing username", config: Config{Host: "127.0.0.1", Database: "app"}},
		{name: "invalid port", config: Config{Host: "127.0.0.1", Port: 65536, Database: "app", Username: "app_user"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newDriverConfig(tt.config); err == nil {
				t.Fatal("newDriverConfig() error = nil, want validation error")
			}
		})
	}
}
