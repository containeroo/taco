package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type mockDialer struct {
	err error
}

func (m *mockDialer) DialContext(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &mockConn{}, nil
}

type mockConn struct{}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestParseEnv(t *testing.T) {
	t.Run("Valid environment variables", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost:5432",
			"INTERVAL":       "1s",
			"DIAL_TIMEOUT":   "1s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		expected := Vars{
			TargetName:  "database",
			Address:     "localhost:5432",
			Interval:    1 * time.Second,
			DialTimeout: 1 * time.Second,
		}

		envVars, err := parseEnv(getenv)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if envVars != expected {
			t.Errorf("Expected %v, got %v", expected, envVars)
		}
	})

	t.Run("Missing TARGET_NAME", func(t *testing.T) {
		env := map[string]string{
			"TARGET_ADDRESS": "localhost:5432",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseEnv(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("Invalid INTERVAL", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost:5432",
			"INTERVAL":       "invalid",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseEnv(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("Missing port in TARGET_ADDRESS", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseEnv(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestCheckConnection(t *testing.T) {
	t.Run("Successful connection", func(t *testing.T) {
		dialer := &mockDialer{}
		ctx := context.Background()
		err := checkConnection(ctx, dialer.DialContext, "localhost:5432", 1*time.Second)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Failed connection", func(t *testing.T) {
		dialer := &mockDialer{err: errors.New("connection error")}
		ctx := context.Background()
		err := checkConnection(ctx, dialer.DialContext, "localhost:5432", 1*time.Second)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestRunCheckerLoop(t *testing.T) {
	t.Run("Service becomes ready", func(t *testing.T) {
		envVars := Vars{
			TargetName:  "database",
			Address:     "localhost:5432",
			Interval:    1 * time.Second,
			DialTimeout: 1 * time.Second,
		}

		dialer := &mockDialer{}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := runLoop(ctx, envVars, dialer.DialContext, &output)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Waiting for database to become ready at localhost:5432...\ndatabase OK ✓"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})

	t.Run("Service fails to become ready", func(t *testing.T) {
		envVars := Vars{
			TargetName:  "database",
			Address:     "localhost:5432",
			Interval:    1 * time.Second,
			DialTimeout: 1 * time.Second,
		}

		dialer := &mockDialer{err: errors.New("connection error")}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := runLoop(ctx, envVars, dialer.DialContext, &output)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Waiting for database: connection error"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("Successful run", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "google.com:80",
			"INTERVAL":       "1s",
			"DIAL_TIMEOUT":   "1s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := run(ctx, getenv, &output, defaultDialTCPContext)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Waiting for database to become ready at google.com:80...\ndatabase OK ✓"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})

	t.Run("Failed connection", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost:5432",
			"INTERVAL":       "1s",
			"DIAL_TIMEOUT":   "1s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := run(ctx, getenv, &output, defaultDialTCPContext)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "connect: connection refused"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})
}
