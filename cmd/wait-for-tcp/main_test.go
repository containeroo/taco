package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
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
			t.Errorf("Expected '%v', got '%v'", expected, envVars)
		}
	})

	t.Run("Missing TARGET_NAME", func(t *testing.T) {
		env := map[string]string{
			"TARGET_ADDRESS": "localhost:5432",
		}

		getenv := func(key string) string {
			return env[key]
		}

		if _, err := parseEnv(getenv); err == nil {
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

		if _, err := parseEnv(getenv); err == nil {
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

		if _, err := parseEnv(getenv); err == nil {
			t.Error("Expected error but got none")
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
		} else {
			fmt.Printf("Expected error occurred: %v\n", err)
		}
	})
}

func TestRunLoop(t *testing.T) {
	t.Run("Target becomes ready", func(t *testing.T) {
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

		expected := "msg=\"Target became ready ✓\""
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain '%q' but got '%q'", expected, output.String())
		}
	})

	t.Run("Target fails to become ready", func(t *testing.T) {
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

		expected := "\"Connection attempt failed ✗\""
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain '%q' but got '%q'", expected, output.String())
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

		expected := fmt.Sprintf("msg=\"Target became ready ✓\" target_name=%s address=%s", envVars.TargetName, envVars.TargetAddress)
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Expected output to contain '%q' but got '%q'", expected, output.String())
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
