package gbase

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	mysql "github.com/WormW/gbase8s-dolphin-go/driver"
)

type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string

	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func Open(ctx context.Context, config Config) (*sql.DB, error) {
	driverConfig, err := newDriverConfig(config)
	if err != nil {
		return nil, err
	}

	connector, err := mysql.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("create GBase connector: %w", err)
	}

	db := sql.OpenDB(connector)
	configurePool(db, config)
	if err := Probe(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Probe(ctx context.Context, db *sql.DB) error {
	var value int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&value); err != nil {
		return fmt.Errorf("probe GBase with SELECT 1: %w", err)
	}
	if value != 1 {
		return fmt.Errorf("probe GBase with SELECT 1: got %d", value)
	}
	return nil
}

func newDriverConfig(config Config) (*mysql.Config, error) {
	if strings.TrimSpace(config.Host) == "" {
		return nil, fmt.Errorf("GBase host is required")
	}
	if strings.TrimSpace(config.Database) == "" {
		return nil, fmt.Errorf("GBase database is required")
	}
	if config.Username == "" {
		return nil, fmt.Errorf("GBase username is required")
	}

	port := config.Port
	if port == 0 {
		port = 3308
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("GBase port %d is outside 1..65535", port)
	}

	driverConfig := mysql.NewConfig()
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(config.Host, strconv.Itoa(port))
	driverConfig.User = config.Username
	driverConfig.Passwd = config.Password
	driverConfig.DBName = config.Database
	driverConfig.ParseTime = true
	driverConfig.InterpolateParams = true
	driverConfig.Params = map[string]string{"charset": "utf8mb4"}
	driverConfig.Timeout = durationOrDefault(config.ConnectTimeout, 5*time.Second)
	driverConfig.ReadTimeout = durationOrDefault(config.ReadTimeout, 30*time.Second)
	driverConfig.WriteTimeout = durationOrDefault(config.WriteTimeout, 30*time.Second)
	return driverConfig, nil
}

func configurePool(db *sql.DB, config Config) {
	maxOpen := config.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = 10
	}
	maxIdle := config.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 2
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(durationOrDefault(config.ConnMaxLifetime, 30*time.Minute))
	db.SetConnMaxIdleTime(durationOrDefault(config.ConnMaxIdleTime, 5*time.Minute))
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}
	return value
}
