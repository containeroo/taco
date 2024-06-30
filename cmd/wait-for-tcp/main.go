package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Vars holds the environment variables required for the service checker.
type Vars struct {
	TargetName    string        // The name of the service to check.
	TargetAddress string        // The address of the service in the format 'host:port'.
	Interval      time.Duration // The interval between connection attempts.
	DialTimeout   time.Duration // The timeout for each connection attempt.
}

// parseEnv retrieves and validates the environment variables required for the service checker.
func parseEnv(getenv func(string) string) (Vars, error) {
	env := Vars{
		TargetName:    getenv("TARGET_NAME"),
		TargetAddress: getenv("TARGET_ADDRESS"),
		Interval:      2 * time.Second, // default interval
		DialTimeout:   2 * time.Second, // default dial timeout
	}

	if env.TargetName == "" || env.TargetAddress == "" {
		return env, fmt.Errorf("TARGET_NAME and TARGET_ADDRESS environment variables are required")
	}

	// Ensure address includes a port
	if !strings.Contains(env.TargetAddress, ":") {
		return env, fmt.Errorf("invalid TARGET_ADDRESS format, must be host:port")
	}

	if intervalStr := getenv("INTERVAL"); intervalStr != "" {
		var err error
		env.Interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			return env, fmt.Errorf("invalid interval value: %s", err)
		}
	}

	if dialTimeoutStr := getenv("DIAL_TIMEOUT"); dialTimeoutStr != "" {
		var err error
		env.DialTimeout, err = time.ParseDuration(dialTimeoutStr)
		if err != nil {
			return env, fmt.Errorf("invalid dial timeout value: %s", err)
		}
	}

	return env, nil
}

// checkConnection attempts to establish a connection to the given address within the specified timeout.
func checkConnection(ctx context.Context, address string, timeout time.Duration) error {
	d := net.Dialer{
		Timeout: timeout,
	}

	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// logMessage logs a message in structured key-value pairs format.
func logMessage(output io.Writer, level string, message string, details map[string]interface{}) {
	logEntry := fmt.Sprintf("ts=%s level=%s msg=%q", time.Now().Format(time.RFC3339), level, message)
	for k, v := range details {
		logEntry += fmt.Sprintf(" %s=%v", k, v)
	}
	fmt.Fprintln(output, logEntry)
}

// runLoop continuously attempts to connect to the specified service
// at regular intervals until the service becomes available or the context
// is cancelled. It handles OS signals for graceful shutdown.
func runLoop(ctx context.Context, envVars Vars, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logMessage(output, "info", "Waiting for service to become ready...", map[string]interface{}{
		"target_name": envVars.TargetName,
		"address":     envVars.TargetAddress,
	})

	// Run the first check immediately
	if err := checkConnection(ctx, envVars.TargetAddress, envVars.DialTimeout); err != nil {
		logMessage(output, "warn", "Initial connection attempt failed", map[string]interface{}{
			"target_name": envVars.TargetName,
			"address":     envVars.TargetAddress,
			"error":       err.Error(),
		})
	} else {
		logMessage(output, "info", "Service became ready", map[string]interface{}{
			"target_name": envVars.TargetName,
			"address":     envVars.TargetAddress,
		})

		return nil
	}

	ticker := time.NewTicker(envVars.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil // Treat context cancellation or timeout as expected behavior
			}
			return ctx.Err()
		case <-ticker.C:
			if err := checkConnection(ctx, envVars.TargetAddress, envVars.DialTimeout); err != nil {
				logMessage(output, "warn", "Connection attempt failed", map[string]interface{}{
					"target_name": envVars.TargetName,
					"address":     envVars.TargetAddress,
					"error":       err.Error(),
				})
				continue
			}

			logMessage(output, "info", "Service became ready", map[string]interface{}{
				"target_name": envVars.TargetName,
				"address":     envVars.TargetAddress,
			})

			cancel()

			return nil
		}
	}
}

// run is the main entry point for running the service checker.
// It initializes the environment variables, sets up the checker, and starts the checking loop.
func run(ctx context.Context, getenv func(string) string, output io.Writer) error {
	envVars, err := parseEnv(getenv)
	if err != nil {
		return err
	}

	return runLoop(ctx, envVars, output)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, os.Getenv, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
