package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseEnv(t *testing.T) {
	t.Run("Valid environment variables", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"TARGET_NAME":           "database",
			"TARGET_ADDRESS":        "localhost:5432",
			"INTERVAL":              "1s",
			"DIAL_TIMEOUT":          "1s",
			"LOG_ADDITIONAL_FIELDS": "true",
		}

		getenv := func(key string) string {
			return env[key]
		}

		cfg, err := parseConfig(getenv)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := Config{
			TargetName:          "database",
			TargetAddress:       "localhost:5432",
			Interval:            1 * time.Second,
			DialTimeout:         1 * time.Second,
			LogAdditionalFields: true,
		}
		if !reflect.DeepEqual(cfg, expected) {
			t.Errorf("Expected %+v, got %+v", expected, cfg)
		}
	})

	t.Run("Invalid INTERVAL", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"INTERVAL": "-s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseConfig(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := fmt.Sprintf("invalid INTERVAL value: time: invalid duration \"%s\"", env["INTERVAL"])
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid DIAL_TIMEOUT", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"DIAL_TIMEOUT": "-s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseConfig(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := fmt.Sprintf("invalid DIAL_TIMEOUT value: time: invalid duration \"%s\"", env["DIAL_TIMEOUT"])
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid LOG_ADDITIONAL_FIELDS", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"LOG_ADDITIONAL_FIELDS": "tr",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseConfig(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := fmt.Sprintf("invalid LOG_ADDITIONAL_FIELDS value: strconv.ParseBool: parsing \"%s\": invalid syntax", env["LOG_ADDITIONAL_FIELDS"])
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})
}

func TestValidateEnv(t *testing.T) {
	t.Run("Valid environment variables", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		err := validateConfig(&cfg)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Generate TARGET_NAME", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetAddress: "localhost:5432",
		}

		err := validateConfig(&cfg)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if cfg.TargetName == "" {
			t.Errorf("Expected TargetName to be generated")
		}

		expected := strings.SplitN(cfg.TargetAddress, ":", 2)[0]
		if cfg.TargetName != expected {
			t.Errorf("Expected target name %q but got %q", expected, cfg.TargetName)
		}
	})

	t.Run("Missing TARGET_ADDRESS", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName: "database",
		}

		err := validateConfig(&cfg)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "TARGET_ADDRESS environment variable is required"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid TARGET_ADDRESS (port)", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost",
		}

		err := validateConfig(&cfg)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "invalid TARGET_ADDRESS format, must be host:port"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid TARGET_ADDRESS (schema)", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "http://localhost:5432",
		}

		err := validateConfig(&cfg)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "TARGET_ADDRESS should not include a schema (http)"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid INTERVAL", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      -1 * time.Second,
		}

		err := validateConfig(&cfg)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "invalid INTERVAL value: interval cannot be negative"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid DIAL_TIMEOUT", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			DialTimeout:   -1 * time.Second,
		}

		err := validateConfig(&cfg)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "invalid DIAL_TIMEOUT value: dial timeout cannot be negative"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})
}

func TestCheckConnection(t *testing.T) {
	t.Run("Successful connection", func(t *testing.T) {
		t.Parallel()

		targetAddress := "127.0.0.1:3306"

		// Setup a mock server to listen on
		lis, err := net.Listen("tcp", targetAddress)
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		dialer := &net.Dialer{
			Timeout: 2 * time.Second,
		}

		ctx := context.Background()
		if err := checkConnection(ctx, dialer, targetAddress); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Failed connection", func(t *testing.T) {
		t.Parallel()

		targetAddress := "localhost:5432"

		dialer := &net.Dialer{
			Timeout: 2 * time.Second,
		}

		ctx := context.Background()
		err := checkConnection(ctx, dialer, targetAddress)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestWaitForTarget(t *testing.T) {
	t.Run("Target is ready", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:27017",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		// Setup a mock server to listen on localhost:5432
		lis, err := net.Listen("tcp", cfg.TargetAddress)
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := slog.New(slog.NewTextHandler(&stdOut, nil))

		// cancel waitForTarget after 2 Seconds
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err = waitForTarget(ctx, cfg, logger)
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := fmt.Sprintf("%s is ready ✓", cfg.TargetName)
		if !strings.Contains(stdOut.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}
	})

	t.Run("Target is not ready", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:6379",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := slog.New(slog.NewTextHandler(&stdOut, nil))

		// cancel waitForTarget after 2 Seconds
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := waitForTarget(ctx, cfg, logger)
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := fmt.Sprintf("%s is not ready ✗", cfg.TargetName)
		if !strings.Contains(stdOut.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}
	})

	t.Run("Successful run after 3 attempts", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:          "PostgreSQL",
			TargetAddress:       "localhost:5432",
			Interval:            50 * time.Millisecond,
			DialTimeout:         50 * time.Millisecond,
			LogAdditionalFields: true,
		}

		var wg sync.WaitGroup
		wg.Add(1)

		var lis net.Listener
		// start listener after 3 seconds
		go func() {
			defer wg.Done() // Mark the WaitGroup as done when the goroutine completes
			time.Sleep(cfg.Interval * 3)
			var err error
			lis, err = net.Listen("tcp", cfg.TargetAddress)
			if err != nil {
				panic("failed to listen: " + err.Error())
			}
			time.Sleep(200 * time.Millisecond) // Ensure runloop get a successful attempt
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var stdOut strings.Builder
		logger := slog.New(slog.NewTextHandler(&stdOut, &slog.HandlerOptions{}))
		logger = logger.With(
			"target_name", cfg.TargetName,
			"target_address", cfg.TargetAddress,
			"interval", cfg.Interval.String(),
			"dial_timeout", cfg.DialTimeout.String(),
			"version", version,
		)

		if err := waitForTarget(ctx, cfg, logger); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		wg.Wait()
		defer lis.Close() // listener must be closed after waiting group is done

		stdOutEntries := strings.Split(strings.TrimSpace(stdOut.String()), "\n")
		// output must be:
		// 0: Waiting for database to become ready...
		// 1: database is not ready ✗
		// 2: database is not ready ✗
		// 3: database is not ready ✗
		// 4: database is ready ✓

		lenExpectedOuts := 5
		if len(stdOutEntries) != lenExpectedOuts {
			t.Errorf("Expected output to contain '%d' lines but got '%d'.", lenExpectedOuts, len(stdOutEntries))
		}

		expected := fmt.Sprintf("Waiting for %s to become ready...", cfg.TargetName)
		if !strings.Contains(stdOutEntries[0], expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOutEntries[0])
		}

		addressPort := strings.Split(cfg.TargetAddress, ":")[1]
		from := 1
		to := 3
		for i := from; i < to; i++ {
			expected = fmt.Sprintf("%s is not ready ✗", cfg.TargetName)
			if !strings.Contains(stdOutEntries[i], expected) {
				t.Errorf("Expected output to contain %q but got %q", expected, stdOutEntries[i])
			}

			expected = fmt.Sprintf("error=\"dial tcp [::1]:%s: connect: connection refused\"", addressPort)
			if !strings.Contains(stdOutEntries[i], expected) {
				t.Errorf("Expected output to contain %q but got %q", expected, stdOutEntries[i])
			}
		}

		expected = fmt.Sprintf("%s is ready ✓", cfg.TargetName)
		if !strings.Contains(stdOutEntries[lenExpectedOuts-1], expected) { // lenExpectedOuts -1 = last element
			t.Errorf("Expected output to contain %q but got %q", expected, stdOutEntries[1])
		}

		expected = fmt.Sprintf("version=%s", version)
		if !strings.Contains(stdOutEntries[lenExpectedOuts-1], expected) { // lenExpectedOuts -1 = last element
			t.Errorf("Expected output to contain %q but got %q", expected, stdOutEntries[1])
		}
	})

	t.Run("Failed connection", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:1433",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := slog.New(slog.NewTextHandler(&stdOut, nil))

		// cancel waitForTarget after 2 Seconds
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		if err := waitForTarget(ctx, cfg, logger); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "connect: connection refused"
		if !strings.Contains(stdOut.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}
	})

	t.Run("Context timeout", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:3306",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var stdOut strings.Builder
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		logger := slog.New(slog.NewTextHandler(&stdOut, nil))

		err := waitForTarget(ctx, cfg, logger)
		if err != nil && err != context.DeadlineExceeded {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "context deadline exceeded"
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Expected error %q but got %q", expected, err.Error())
		}
	})

	t.Run("Context cancel", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			TargetName:    "database",
			TargetAddress: "localhost:9042",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())

		logger := slog.New(slog.NewTextHandler(&stdOut, nil))

		// cancel waitForTarget after 1 Seconds
		go func() {
			time.Sleep(1 * time.Second)
			cancel()
		}()

		err := waitForTarget(ctx, cfg, logger)
		// waitForTarget returns nil if context is canceled
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestConcurrentConnections(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TargetName:    "database",
		TargetAddress: "localhost:9200",
		Interval:      1 * time.Second,
		DialTimeout:   1 * time.Second,
	}

	// Setup a mock server to listen on localhost:5432
	lis, err := net.Listen("tcp", cfg.TargetAddress)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	var stdOut strings.Builder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(&stdOut, nil))

	var wg sync.WaitGroup
	numRoutines := 4
	wg.Add(numRoutines)

	for i := 0; i < numRoutines; i++ {
		go func() {
			defer wg.Done()
			err := waitForTarget(ctx, cfg, logger)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	// Simulate context cancel after 2 seconds
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	wg.Wait()

	expected := fmt.Sprintf("%s is ready ✓", cfg.TargetName)
	if !strings.Contains(stdOut.String(), expected) {
		t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
	}
}

func TestRun(t *testing.T) {
	t.Run("Successful run", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost:8091",
			"INTERVAL":       "1s",
			"DIAL_TIMEOUT":   "1s",
		}

		getenv := func(key string) string {
			return env[key]
		}

		// Setup a mock server to listen on localhost:3306
		lis, err := net.Listen("tcp", env["TARGET_ADDRESS"])
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// cancel run after 2 Seconds
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		if err := run(ctx, getenv, &stdOut); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		stdOutEntries := strings.Split(strings.TrimSpace(stdOut.String()), "\n")

		lenExpectedOuts := 2
		if len(stdOutEntries) != lenExpectedOuts {
			t.Errorf("Expected output to contain '%d' lines but got '%d'", lenExpectedOuts, len(stdOutEntries))
		}

		expected := fmt.Sprintf("Waiting for %s to become ready...", env["TARGET_NAME"])
		if !strings.Contains(stdOutEntries[0], expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}

		expected = fmt.Sprintf("%s is ready ✓", env["TARGET_NAME"])
		if !strings.Contains(stdOutEntries[lenExpectedOuts-1], expected) { // lenExpectedOuts -1 = last element
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}
	})

	t.Run("Failed run due to invalid address", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "localhost",
		}

		getenv := func(key string) string {
			return env[key]
		}

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := run(ctx, getenv, &stdOut)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "invalid TARGET_ADDRESS format, must be host:port"
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Expected error %q but got %q", expected, err.Error())
		}
	})

	t.Run("LogAdditionalFields set to true", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"TARGET_NAME":           "database",
			"TARGET_ADDRESS":        "localhost:8092",
			"INTERVAL":              "1s",
			"DIAL_TIMEOUT":          "1s",
			"LOG_ADDITIONAL_FIELDS": "true",
		}

		getenv := func(key string) string {
			return env[key]
		}

		// Setup a mock server to listen on localhost:8092
		lis, err := net.Listen("tcp", env["TARGET_ADDRESS"])
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		var stdOut strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// cancel run after 2 Seconds
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		if err := run(ctx, getenv, &stdOut); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		stdOutEntries := strings.Split(strings.TrimSpace(stdOut.String()), "\n")

		lenExpectedOuts := 2
		if len(stdOutEntries) != lenExpectedOuts {
			t.Errorf("Expected output to contain '%d' lines but got '%d'", lenExpectedOuts, len(stdOutEntries))
		}

		expected := fmt.Sprintf("Waiting for %s to become ready...", env["TARGET_NAME"])
		if !strings.Contains(stdOutEntries[0], expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}

		expected = fmt.Sprintf("%s is ready ✓", env["TARGET_NAME"])
		if !strings.Contains(stdOutEntries[lenExpectedOuts-1], expected) { // lenExpectedOuts -1 = last element
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}

		expected = fmt.Sprintf("version=%s", version)
		if !strings.Contains(stdOutEntries[lenExpectedOuts-1], expected) { // lenExpectedOuts -1 = last element
			t.Errorf("Expected output to contain %q but got %q", expected, stdOut.String())
		}
	})
}
