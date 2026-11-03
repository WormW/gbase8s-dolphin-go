package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	gbase "github.com/WormW/gbase8s-dolphin-go"
)

func main() {
	write := flag.Bool("write", false, "create a temporary table to verify binary values and auto-increment IDs")
	flag.Parse()

	if err := run(*write); err != nil {
		fmt.Fprintln(os.Stderr, "smoke test failed:", err)
		os.Exit(1)
	}
}

func run(write bool) error {
	port, err := envPort("GBASE_PORT", 3308)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	db, err := gbase.Open(ctx, gbase.Config{
		Host:     os.Getenv("GBASE_HOST"),
		Port:     port,
		Database: os.Getenv("GBASE_DATABASE"),
		Username: os.Getenv("GBASE_USER"),
		Password: os.Getenv("GBASE_PASSWORD"),
	})
	if err != nil {
		return err
	}
	defer db.Close()

	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return fmt.Errorf("read server version: %w", err)
	}
	fmt.Println("server:", version)

	if err := checkNullRow(ctx, db); err != nil {
		return err
	}
	fmt.Println("NULL columns: ok")

	if err := checkString(ctx, db); err != nil {
		return err
	}
	fmt.Println("string parameters: ok")

	if write {
		if err := checkWrite(ctx, db); err != nil {
			return err
		}
		fmt.Println("binary and auto-increment ID: ok")
	} else {
		fmt.Println("write checks: skipped (use -write to enable)")
	}
	return nil
}

func checkNullRow(ctx context.Context, db *sql.DB) error {
	var first, third string
	var second, fourth sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT 'a', NULL, 'b', NULL").Scan(&first, &second, &third, &fourth); err != nil {
		return fmt.Errorf("NULL row check: %w", err)
	}
	if first != "a" || third != "b" || second.Valid || fourth.Valid {
		return fmt.Errorf("NULL row check: unexpected values %q, %v, %q, %v", first, second, third, fourth)
	}
	return nil
}

func checkString(ctx context.Context, db *sql.DB) error {
	const input = "single ', backslash \\, Chinese 中文"
	var output string
	if err := db.QueryRowContext(ctx, "SELECT ?", input).Scan(&output); err != nil {
		return fmt.Errorf("string parameter check: %w", err)
	}
	if output != input {
		return fmt.Errorf("string parameter check: got %q, want %q", output, input)
	}
	return nil
}

func checkWrite(ctx context.Context, db *sql.DB) (returnErr error) {
	table := fmt.Sprintf("gbase_dolphin_smoke_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+" (id BIGINT AUTO_INCREMENT PRIMARY KEY, text_value VARCHAR(512), binary_value BLOB, nullable_value VARCHAR(64))"); err != nil {
		return fmt.Errorf("create smoke table: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanupCtx, "DROP TABLE IF EXISTS "+table); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("drop smoke table: %w", err)
		}
	}()

	text := "quote ', slash \\, 中文"
	binary := []byte{0x00, 0x01, 0x27, 0x5c, 0xff}
	result, err := db.ExecContext(ctx, "INSERT INTO "+table+" (text_value, binary_value, nullable_value) VALUES (?, ?, NULL)", text, binary)
	if err != nil {
		return fmt.Errorf("insert smoke row: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read auto-increment ID: %w", err)
	}
	if id <= 0 {
		return fmt.Errorf("read auto-increment ID: got %d", id)
	}

	var gotText string
	var gotBinary []byte
	var gotNull sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT text_value, binary_value, nullable_value FROM "+table+" WHERE id = ?", id).Scan(&gotText, &gotBinary, &gotNull); err != nil {
		return fmt.Errorf("read smoke row: %w", err)
	}
	if gotText != text || !bytes.Equal(gotBinary, binary) || gotNull.Valid {
		return fmt.Errorf("read smoke row: values did not round-trip")
	}
	return nil
}

func envPort(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be an integer in 1..65535", name)
	}
	return port, nil
}
