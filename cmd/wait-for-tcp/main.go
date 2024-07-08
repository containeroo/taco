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

const version = "0.0.16"

// Vars holds the environment variables required for the target checker.
type Vars struct {
	TargetName    string        // The name of the target to check.
	TargetAddress string        // The address of the target in the format 'host:port'.
	Interval      time.Duration // The interval between connection attempts.
	DialTimeout   time.Duration // The timeout for each connection attempt.
	LogFields     bool          // Logs additional fields
}

// parseEnv retrieves and validates the environment variables required for the target checker.
func parseEnv(getenv func(string) string) (Vars, error) {
	env := Vars{
		TargetName:    getenv("TARGET_NAME"),
		TargetAddress: getenv("TARGET_ADDRESS"),
		Interval:      2 * time.Second, // default interval
		DialTimeout:   1 * time.Second, // default dial timeout
		LogFields:     false,
	}

	if env.TargetName == "" {
		return Vars{}, fmt.Errorf("TARGET_NAME environment variable is required")
	}

	if env.TargetAddress == "" {
		return Vars{}, fmt.Errorf("TARGET_ADDRESS environment variable is required")
	}

	if schema := strings.SplitN(env.TargetAddress, "://", 2); len(schema) > 1 {
		return Vars{}, fmt.Errorf("TARGET_ADDRESS should not include a schema (%s)", schema[0])
	}

	if !strings.Contains(env.TargetAddress, ":") {
		return Vars{}, fmt.Errorf("invalid TARGET_ADDRESS format, must be host:port")
	}

	if intervalStr := getenv("INTERVAL"); intervalStr != "" {
		var err error
		env.Interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			return Vars{}, fmt.Errorf("invalid INTERVAL value: %s", err)
		}
	}

	if dialTimeoutStr := getenv("DIAL_TIMEOUT"); dialTimeoutStr != "" {
		var err error
		env.DialTimeout, err = time.ParseDuration(dialTimeoutStr)
		if err != nil {
			return Vars{}, fmt.Errorf("invalid DIAL_TIMEOUT value: %s", err)
		}
	}

	if logFieldsStr := getenv("LOG_FIELDS"); logFieldsStr != "" {
		var err error
		env.LogFields, err = strconv.ParseBool(logFieldsStr)
		if err != nil {
			return Vars{}, fmt.Errorf("invalid LOG_FIELDS value: %s", err)
		}
	}
	return env, nil
}

// checkConnection attempts to establish a connection to the given address within the specified timeout.
func checkConnection(ctx context.Context, dialer *net.Dialer, address string) error {
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// runLoop continuously attempts to connect to the specified service until the service becomes available or the context is cancelled.
func runLoop(ctx context.Context, envVars Vars, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if envVars.LogFields {
		logger = logger.With(
			"target_name", envVars.TargetName,
			"target_address", envVars.TargetAddress,
			"interval", envVars.Interval.String(),
			"dial_timeout", envVars.DialTimeout.String(),
			"version", version,
		)
	}

	logger.Info(fmt.Sprintf("Waiting for %s to become ready...", envVars.TargetName))

	dialer := &net.Dialer{
		Timeout: envVars.DialTimeout,
	}

	for {
		err := checkConnection(ctx, dialer, envVars.TargetAddress)
		if err == nil {
			logger.Info(fmt.Sprintf("%s is ready ✓", envVars.TargetName))
			return nil
		}

		logger.Warn(fmt.Sprintf("%s is not ready ✗", envVars.TargetName), "error", err.Error())

		select {
		case <-time.After(envVars.Interval):
			// Continue to the next connection attempt after the interval
		case <-ctx.Done():
			err := ctx.Err()
			if ctx.Err() == context.Canceled {
				return nil // Treat context cancellation as expected behavior
			}
			return err
		}
	}
}

// run is the main entry point for running the target checker.
func run(ctx context.Context, getenv func(string) string, stdOut io.Writer) error {
	envVars, err := parseEnv(getenv)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(stdOut, nil))

	return runLoop(ctx, envVars, logger)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
