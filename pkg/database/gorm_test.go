package database

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "disabled config is always valid",
			cfg:     Config{Enabled: false},
			wantErr: false,
		},
		{
			name:    "invalid dialect",
			cfg:     Config{Enabled: true, Dialect: "oracle"},
			wantErr: true,
			errMsg:  "invalid dialect",
		},
		{
			name: "mysql missing host",
			cfg: Config{
				Enabled: true, Dialect: "mysql",
				Port: "3306", User: "root", Password: "pass", DBName: "testdb",
			},
			wantErr: true,
			errMsg:  "missing required fields",
		},
		{
			name: "mysql missing password",
			cfg: Config{
				Enabled: true, Dialect: "mysql",
				Host: "localhost", Port: "3306", User: "root", DBName: "testdb",
			},
			wantErr: true,
			errMsg:  "missing required fields",
		},
		{
			name: "postgres valid",
			cfg: Config{
				Enabled: true, Dialect: "postgres",
				Host: "localhost", Port: "5432", User: "pg", Password: "pass", DBName: "mydb",
			},
			wantErr: false,
		},
		{
			name: "mysql valid",
			cfg: Config{
				Enabled: true, Dialect: "mysql",
				Host: "localhost", Port: "3306", User: "root", Password: "pass", DBName: "mydb",
			},
			wantErr: false,
		},
		{
			name:    "sqlite valid without host fields",
			cfg:     Config{Enabled: true, Dialect: "sqlite"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	cfg := Config{Dialect: "postgres"}
	cfg.applyDefaults()

	if cfg.MaxIdle != 10 {
		t.Errorf("expected MaxIdle=10, got %d", cfg.MaxIdle)
	}
	if cfg.MaxOpen != 100 {
		t.Errorf("expected MaxOpen=100, got %d", cfg.MaxOpen)
	}
	if cfg.MaxLifetime != time.Hour {
		t.Errorf("expected MaxLifetime=1h, got %v", cfg.MaxLifetime)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected SSLMode=disable for postgres, got %q", cfg.SSLMode)
	}

	cfg2 := Config{Dialect: "mysql"}
	cfg2.applyDefaults()
	if cfg2.SSLMode != "" {
		t.Errorf("expected empty SSLMode for mysql, got %q", cfg2.SSLMode)
	}

	cfg3 := Config{Dialect: "postgres", MaxIdle: 5, MaxOpen: 50, MaxLifetime: 30 * time.Minute, SSLMode: "require"}
	cfg3.applyDefaults()
	if cfg3.MaxIdle != 5 || cfg3.MaxOpen != 50 || cfg3.MaxLifetime != 30*time.Minute || cfg3.SSLMode != "require" {
		t.Error("applyDefaults should not overwrite existing values")
	}
}

func TestBuildDSN_MySQL(t *testing.T) {
	cfg := Config{
		Dialect:  "mysql",
		Host:     "dbhost",
		Port:     "3306",
		User:     "admin",
		Password: "secret",
		DBName:   "mydb",
	}

	dsn, err := buildDSN(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(dsn, "admin:secret@tcp(dbhost:3306)/mydb?") {
		t.Errorf("unexpected MySQL DSN prefix: %s", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Error("expected charset=utf8mb4 in DSN")
	}
	if !strings.Contains(dsn, "parseTime=true") {
		t.Error("expected parseTime=true in DSN")
	}
}

func TestBuildDSN_PostgreSQL(t *testing.T) {
	cfg := Config{
		Dialect:  "postgres",
		Host:     "pghost",
		Port:     "5432",
		User:     "pguser",
		Password: "pgpass",
		DBName:   "pgdb",
		SSLMode:  "require",
	}

	dsn, err := buildDSN(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "host=pghost port=5432 user=pguser password=pgpass dbname=pgdb sslmode=require"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_PostgreSQL_DefaultSSL(t *testing.T) {
	cfg := Config{
		Dialect:  "postgres",
		Host:     "pghost",
		Port:     "5432",
		User:     "pguser",
		Password: "pgpass",
		DBName:   "pgdb",
	}

	dsn, _ := buildDSN(cfg)
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("expected sslmode=disable as default, got %q", dsn)
	}
}

func TestBuildDSN_SQLite(t *testing.T) {
	cfg := Config{Dialect: "sqlite", DSN: "/tmp/test.db"}
	dsn, err := buildDSN(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dsn != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %q", dsn)
	}

	cfg2 := Config{Dialect: "sqlite"}
	dsn2, _ := buildDSN(cfg2)
	if dsn2 != "file::memory:?cache=shared" {
		t.Errorf("expected in-memory default, got %q", dsn2)
	}
}

func TestBuildDSN_UnsupportedDialect(t *testing.T) {
	cfg := Config{Dialect: "oracle"}
	_, err := buildDSN(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported dialect")
	}
}
