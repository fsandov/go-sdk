package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fsandov/go-sdk/pkg/logs"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Enabled     bool
	Dialect     string
	DSN         string
	Host        string
	Port        string
	User        string
	Password    string
	DBName      string
	SSLMode     string
	MaxIdle     int
	MaxOpen     int
	MaxLifetime time.Duration
}

var DefaultMySqlConfig = Config{
	Enabled:     true,
	Dialect:     "mysql",
	Host:        os.Getenv("MYSQL_HOST"),
	Port:        os.Getenv("MYSQL_PORT"),
	User:        os.Getenv("MYSQL_USER"),
	Password:    os.Getenv("MYSQL_PASSWORD"),
	DBName:      os.Getenv("MYSQL_DBNAME"),
	MaxIdle:     10,
	MaxOpen:     100,
	MaxLifetime: time.Hour,
}

func (c *Config) applyDefaults() {
	if c.MaxIdle == 0 {
		c.MaxIdle = 10
	}
	if c.MaxOpen == 0 {
		c.MaxOpen = 100
	}
	if c.MaxLifetime == 0 {
		c.MaxLifetime = time.Hour
	}
	if c.SSLMode == "" && c.Dialect == "postgres" {
		c.SSLMode = "disable"
	}
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch c.Dialect {
	case "mysql", "postgres", "sqlite":
	default:
		return errors.New("database: invalid dialect")
	}
	if c.Dialect != "sqlite" {
		if c.Host == "" || c.Port == "" || c.User == "" || c.Password == "" || c.DBName == "" {
			return errors.New("database: missing required fields")
		}
	}
	return nil
}

type Dialect string

const (
	DialectMySQL      Dialect = "mysql"
	DialectPostgreSQL Dialect = "postgres"
	DialectSQLite     Dialect = "sqlite"
)

type Options struct {
	Logger         *logs.Logger
	MaxRetries     int
	RetryInterval  time.Duration
	AutoMigrate    bool
	HealthCheck    bool
	HealthInterval time.Duration
	OnFailure      func(error)
}

func Open(cfg Config, opts *Options) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("database: invalid config: %w", err)
	}
	if opts == nil {
		opts = &Options{}
	}
	if opts.Logger == nil {
		opts.Logger = logs.GetLogger()
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryInterval == 0 {
		opts.RetryInterval = 2 * time.Second
	}
	if opts.HealthInterval == 0 {
		opts.HealthInterval = 30 * time.Second
	}

	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{}
	var db *gorm.DB

	for i := 0; i <= opts.MaxRetries; i++ {
		switch Dialect(cfg.Dialect) {
		case DialectMySQL:
			db, err = gorm.Open(mysql.Open(dsn), gormConfig)
		case DialectPostgreSQL:
			db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		case DialectSQLite:
			db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		default:
			return nil, fmt.Errorf("database: unsupported dialect '%s'", cfg.Dialect)
		}
		if err == nil {
			break
		}
		if i == opts.MaxRetries {
			return nil, fmt.Errorf("database: failed after %d attempts: %w", opts.MaxRetries, err)
		}
		opts.Logger.Warn(context.Background(), "database connection failed, retrying...",
			zap.Error(err),
			zap.Int("attempt", i+1),
			zap.Duration("retry_interval", opts.RetryInterval),
		)
		time.Sleep(opts.RetryInterval)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = sqlDB.PingContext(ctx)
	cancel()
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	if opts.HealthCheck {
		go healthChecker(db, opts.HealthInterval, opts.Logger, opts.OnFailure)
	}

	opts.Logger.Info(context.Background(), "database connection established",
		zap.String("dialect", cfg.Dialect),
		zap.String("host", cfg.Host),
		zap.String("db", cfg.DBName),
		zap.String("status", "connected"),
	)
	return db, nil
}

func buildDSN(cfg Config) (string, error) {
	switch Dialect(cfg.Dialect) {
	case DialectMySQL:
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
		), nil
	case DialectPostgreSQL:
		sslmode := cfg.SSLMode
		if sslmode == "" {
			sslmode = "disable"
		}
		return fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, sslmode,
		), nil
	case DialectSQLite:
		if cfg.DSN != "" {
			return cfg.DSN, nil
		}
		return "file::memory:?cache=shared", nil
	default:
		return "", fmt.Errorf("database: unsupported dialect '%s'", cfg.Dialect)
	}
}

func healthChecker(db *gorm.DB, interval time.Duration, logger *logs.Logger, notify func(error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		<-ticker.C
		if err := db.Exec("SELECT 1").Error; err != nil {
			logger.Error(context.Background(), "database health check failed", zap.Error(err))
			if notify != nil {
				notify(err)
			}
		}
	}
}
