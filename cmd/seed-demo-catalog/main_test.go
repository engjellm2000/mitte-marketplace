package main

import "testing"

func TestValidateSeedMode(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		enabled     string
		wantErr     bool
	}{
		{name: "development with explicit opt-in", environment: "development", enabled: "1"},
		{name: "production is rejected", environment: "production", enabled: "1", wantErr: true},
		{name: "missing environment is rejected", enabled: "1", wantErr: true},
		{name: "missing explicit opt-in is rejected", environment: "development", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateSeedMode(test.environment, test.enabled); (err != nil) != test.wantErr {
				t.Fatalf("validateSeedMode() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateDemoDatabase(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
		wantErr     bool
	}{
		{name: "local database", databaseURL: "postgres://user:pass@localhost:5432/shop?sslmode=disable"},
		{name: "loopback IPv4", databaseURL: "postgres://user:pass@127.0.0.1:5432/shop"},
		{name: "loopback IPv6", databaseURL: "postgres://user:pass@[::1]:5432/shop"},
		{name: "loopback IPv4 alias", databaseURL: "postgres://user:pass@127.0.0.2:5432/shop"},
		{name: "remote database is rejected", databaseURL: "postgres://user:pass@db.example.com:5432/shop", wantErr: true},
		{name: "remote host override is rejected", databaseURL: "postgres://user:pass@localhost:5432/shop?host=db.example.com", wantErr: true},
		{name: "remote host address override is rejected", databaseURL: "postgres://user:pass@localhost:5432/shop?hostaddr=192.0.2.1", wantErr: true},
		{name: "service file is rejected", databaseURL: "postgres://user:pass@localhost:5432/shop?service=production", wantErr: true},
		{name: "non-PostgreSQL URL is rejected", databaseURL: "https://localhost/shop", wantErr: true},
		{name: "invalid database URL is rejected", databaseURL: "not-a-database-url", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateDemoDatabase(test.databaseURL); (err != nil) != test.wantErr {
				t.Fatalf("validateDemoDatabase() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
