package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

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
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		envVars, err := parseEnv(getenv)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if envVars != expected {
			t.Errorf("Expected %q, got %q", expected, envVars)
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

		expected := "TARGET_NAME environment variable is required"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Missing TARGET_ADDRESS", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME": "database",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseEnv(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "TARGET_ADDRESS environment variable is required"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid TARGET_ADDRESS (port)", func(t *testing.T) {
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

		expected := "invalid TARGET_ADDRESS format, must be host:port"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})

	t.Run("Invalid TARGET_ADDRESS (schema)", func(t *testing.T) {
		env := map[string]string{
			"TARGET_NAME":    "database",
			"TARGET_ADDRESS": "http://localhost:5432",
		}

		getenv := func(key string) string {
			return env[key]
		}

		_, err := parseEnv(getenv)
		if err == nil {
			t.Error("Expected error but got none")
		}

		expected := "TARGET_ADDRESS should not include a schema (http)"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
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

		expected := "invalid interval value: time: invalid duration \"invalid\""
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
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

		expected := "invalid TARGET_ADDRESS format, must be host:port"
		if err.Error() != expected {
			t.Errorf("Expected output %q but got %q", expected, err.Error())
		}
	})
}

func TestCheckConnection(t *testing.T) {
	t.Run("Successful connection", func(t *testing.T) {
		// Setup a mock server to listen on
		targetAddress := "127.0.0.1:5432"
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
		targetAddress := "invalid.nonexistent:5432"

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

func TestRunLoop(t *testing.T) {
	t.Run("Target is ready", func(t *testing.T) {
		envVars := Vars{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		// Setup a mock server to listen on localhost:5432
		lis, err := net.Listen("tcp", envVars.TargetAddress)
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err = runLoop(ctx, envVars, &output)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Target is ready ✓"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})

	t.Run("Target is not ready", func(t *testing.T) {
		envVars := Vars{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		err := runLoop(ctx, envVars, &output)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Target is not ready ✗"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("Successful run", func(t *testing.T) {
		envVars := Vars{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		// Setup a mock server to listen on
		lis, err := net.Listen("tcp", envVars.TargetAddress)
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		if err := runLoop(ctx, envVars, &output); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "Target is ready ✓"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})

	t.Run("Successful run after 3 attempts", func(t *testing.T) {
		envVars := Vars{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var wg sync.WaitGroup
		wg.Add(1)

		var lis net.Listener
		go func() {
			defer wg.Done() // Mark the WaitGroup as done when the goroutine completes
			time.Sleep(envVars.Interval * 3)
			var err error
			lis, err = net.Listen("tcp", envVars.TargetAddress)
			if err != nil {
				panic("failed to listen: " + err.Error())
			}
			time.Sleep(1 * time.Second) // Ensure runloop get a successful attemp
		}()

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := runLoop(ctx, envVars, &output); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		wg.Wait()

		defer lis.Close()

		entries := strings.Split(strings.TrimSpace(output.String()), "\n")
		if len(entries) != 5 {
			t.Errorf("Expected output to contain '5' lines but got '%d'", len(entries))
		}

		expected := fmt.Sprintf("Starting 'wait-for-tcp' (version: %s", version)
		if !strings.Contains(entries[0], expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, entries[3])
		}

		expected = "Target is not ready ✗"
		for i := 1; i < 3; i++ {
			if !strings.Contains(entries[i], expected) {
				t.Errorf("Expected output to contain %q but got %q", expected, entries[i])
			}
		}

		expected = "Target is ready ✓"
		if !strings.Contains(entries[4], expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, entries[3])
		}
	})

	t.Run("Failed connection", func(t *testing.T) {
		envVars := Vars{
			TargetName:    "database",
			TargetAddress: "localhost:5432",
			Interval:      1 * time.Second,
			DialTimeout:   1 * time.Second,
		}

		var output strings.Builder
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		if err := runLoop(ctx, envVars, &output); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "connect: connection refused"
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain %q but got %q", expected, output.String())
		}
	})
}
