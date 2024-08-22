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

const version = "0.0.24"

// Config holds the required environment variables.
type Config struct {
	TargetName          string        // The name of the target to check.
	TargetAddress       string        // The address of the target in the format 'host:port'.
	Interval            time.Duration // The interval between connection attempts.
	DialTimeout         time.Duration // The timeout for each connection attempt.
	LogAdditionalFields bool          // Whether to log the fields in the log message.
}

// parseConfig retrieves and parses the required environment variables.
// Provides default values if the environment variables are not set.
func parseConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		TargetName:          getenv("TARGET_NAME"),
		TargetAddress:       getenv("TARGET_ADDRESS"),
		Interval:            2 * time.Second, // default interval
		DialTimeout:         1 * time.Second, // default dial timeout
		LogAdditionalFields: false,
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

	if logFieldsStr := getenv("LOG_ADDITIONAL_FIELDS"); logFieldsStr != "" {
		var err error
		cfg.LogAdditionalFields, err = strconv.ParseBool(logFieldsStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid LOG_ADDITIONAL_FIELDS value: %s", err)
		}
	}

	return cfg, nil
}

// validateConfig checks if the configuration is valid.
func validateConfig(cfg *Config) error {
	if cfg.TargetAddress == "" {
		return fmt.Errorf("TARGET_ADDRESS environment variable is required")
	}

	if schema := strings.SplitN(cfg.TargetAddress, "://", 2); len(schema) > 1 {
		return fmt.Errorf("TARGET_ADDRESS should not include a schema (%s)", schema[0])
	}

	if !strings.Contains(cfg.TargetAddress, ":") {
		return fmt.Errorf("invalid TARGET_ADDRESS format, must be host:port")
	}

	if cfg.TargetName == "" {
		// if the target name is not set, try to infer it from the host part of the target address
		hostPart := strings.SplitN(cfg.TargetAddress, ":", 2)[0] // get the host part
		hostSegments := strings.SplitN(hostPart, ".", 2)         // get the first part of the host
		cfg.TargetName = hostSegments[0]
	}

	if cfg.Interval < 0 {
		return fmt.Errorf("invalid INTERVAL value: interval cannot be negative")
	}

	if cfg.DialTimeout < 0 {
		return fmt.Errorf("invalid DIAL_TIMEOUT value: dial timeout cannot be negative")
	}

	return nil
}

// setupLogger configures the logger based on the configuration
func setupLogger(cfg Config, output io.Writer) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{}

	if cfg.LogAdditionalFields {
		return slog.New(slog.NewTextHandler(output, handlerOpts)).With(
			slog.String("target_address", cfg.TargetAddress),
			slog.String("interval", cfg.Interval.String()),
			slog.String("dial_timeout", cfg.DialTimeout.String()),
			slog.String("version", version),
		)
	}

	// If logAdditionalFields is false, remove the error attribute from the handler
	handlerOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "error" {
			return slog.Attr{}
		}
		return a
	}

	return slog.New(slog.NewTextHandler(output, handlerOpts))
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

// run is the main entry point.
// It sets up signal handling, configuration parsing, and starts the waitForTarget loop.
func run(ctx context.Context, getenv func(string) string, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := parseConfig(getenv)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	logger := setupLogger(cfg, output)

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
