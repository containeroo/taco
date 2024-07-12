package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "0.0.22"

// Config holds the required environment variables.
type Config struct {
	TargetName    string        // The name of the target to check.
	TargetAddress string        // The address of the target in the format 'host:port'.
	Interval      time.Duration // The interval between connection attempts.
	DialTimeout   time.Duration // The timeout for each connection attempt.
	LogFields     bool          // Whether to log the fields in the log message.
}

// parseConfig retrieves and parses the required environment variables.
func parseConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		TargetName:    getenv("TARGET_NAME"),
		TargetAddress: getenv("TARGET_ADDRESS"),
		Interval:      2 * time.Second, // default interval
		DialTimeout:   1 * time.Second, // default dial timeout
		LogFields:     false,
	}

	if intervalStr := getenv("INTERVAL"); intervalStr != "" {
		var err error
		cfg.Interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid INTERVAL value: %s", err)
		}
	}

	if dialTimeoutStr := getenv("DIAL_TIMEOUT"); dialTimeoutStr != "" {
		var err error
		cfg.DialTimeout, err = time.ParseDuration(dialTimeoutStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DIAL_TIMEOUT value: %s", err)
		}
	}

	if logFieldsStr := getenv("LOG_FIELDS"); logFieldsStr != "" {
		var err error
		cfg.LogFields, err = strconv.ParseBool(logFieldsStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid LOG_FIELDS value: %s", err)
		}
	}

	return cfg, nil
}

// validateConfig check if the configuration is valid.
func validateConfig(cfg *Config) error {
	if cfg.TargetName == "" {
		return fmt.Errorf("TARGET_NAME environment variable is required")
	}

	if cfg.TargetAddress == "" {
		return fmt.Errorf("TARGET_ADDRESS environment variable is required")
	}

	if schema := strings.SplitN(cfg.TargetAddress, "://", 2); len(schema) > 1 {
		return fmt.Errorf("TARGET_ADDRESS should not include a schema (%s)", schema[0])
	}

	if !strings.Contains(cfg.TargetAddress, ":") {
		return fmt.Errorf("invalid TARGET_ADDRESS format, must be host:port")
	}

	if cfg.Interval < 0 {
		return fmt.Errorf("invalid INTERVAL value: interval cannot be negative")
	}

	if cfg.DialTimeout < 0 {
		return fmt.Errorf("invalid DIAL_TIMEOUT value: dial timeout cannot be negative")
	}

	return nil
}

// checkConnection tries to establish a connection to the given address.
func checkConnection(ctx context.Context, dialer *net.Dialer, address string) error {
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// waitForTarget continuously attempts to connect to the specified target until it becomes available or the context is canceled.
func waitForTarget(ctx context.Context, cfg Config, logger *slog.Logger) error {
	logger.Info(fmt.Sprintf("Waiting for %s to become ready...", cfg.TargetName))

	dialer := &net.Dialer{
		Timeout: cfg.DialTimeout,
	}

	for {
		err := checkConnection(ctx, dialer, cfg.TargetAddress)
		if err == nil {
			logger.Info(fmt.Sprintf("%s is ready ✓", cfg.TargetName))
			return nil
		}

		logger.Warn(fmt.Sprintf("%s is not ready ✗", cfg.TargetName), "error", err.Error())

		select {
		case <-time.After(cfg.Interval):
			// Continue to the next connection attempt after the interval
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return nil // Treat context cancellation as expected behavior
			}
			return ctx.Err()
		}
	}
}

// run is the main entry point
func run(ctx context.Context, getenv func(string) string, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := parseConfig(getenv)
	if err != nil {
		return err
	}

	if err := validateConfig(&cfg); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(output, nil))
	if cfg.LogFields {
		logger = logger.With(
			"target_name", cfg.TargetName,
			"target_address", cfg.TargetAddress,
			"interval", cfg.Interval.String(),
			"dial_timeout", cfg.DialTimeout.String(),
			"version", version,
		)
	}

	return waitForTarget(ctx, cfg, logger)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
