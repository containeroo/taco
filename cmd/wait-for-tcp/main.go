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
	TargetName  string        // The name of the service to check.
	Address     string        // The address of the service in the format 'host:port'.
	Interval    time.Duration // The interval between connection attempts.
	DialTimeout time.Duration // The timeout for each connection attempt.
}

// parseEnv retrieves and validates the environment variables required for the service checker.
func parseEnv(getenv func(string) string) (Vars, error) {
	env := Vars{
		TargetName:  getenv("TARGET_NAME"),
		Address:     getenv("TARGET_ADDRESS"),
		Interval:    2 * time.Second, // default interval
		DialTimeout: 2 * time.Second, // default dial timeout
	}

	if env.TargetName == "" || env.Address == "" {
		return env, fmt.Errorf("TARGET_NAME and TARGET_ADDRESS environment variables are required")
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

	// Ensure address includes a port
	if !strings.Contains(env.Address, ":") {
		return env, fmt.Errorf("invalid TARGET_ADDRESS format, must be host:port")
	}

	return env, nil
}

// DialFunc defines the function signature for dialing a TCP connection.
type DialFunc func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error)

// defaultDialTCPContext is the default implementation for dialing a TCP connection.
func defaultDialTCPContext(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{
		Timeout: timeout,
	}
	return d.DialContext(ctx, network, address)
}

// checkConnection attempts to establish a connection to the given address within the specified timeout.
func checkConnection(ctx context.Context, dial DialFunc, address string, timeout time.Duration) error {
	conn, err := dial(ctx, "tcp", address, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// logMessage logs a message in structured key-value pairs format.
func logMessage(output io.Writer, message string, details map[string]interface{}) {
	logEntry := fmt.Sprintf("ts=%s level=info msg=%q", time.Now().Format(time.RFC3339), message)
	for k, v := range details {
		logEntry += fmt.Sprintf(" %s=%v", k, v)
	}
	fmt.Fprintln(output, logEntry)
}

// runLoop continuously attempts to connect to the specified service
// at regular intervals until the service becomes available or the context
// is cancelled. It handles OS signals for graceful shutdown.
func runLoop(ctx context.Context, envVars Vars, dial DialFunc, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logMessage(output, "Waiting for service to become ready...", map[string]interface{}{
		"target_name": envVars.TargetName,
		"address":     envVars.Address,
	})

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
			if err := checkConnection(ctx, dial, envVars.Address, envVars.DialTimeout); err != nil {
				logMessage(output, "Connection attempt failed", map[string]interface{}{
					"target_name": envVars.TargetName,
					"address":     envVars.Address,
					"error":       err.Error(),
				})
				continue
			}

			logMessage(output, "Service became ready", map[string]interface{}{
				"target_name": envVars.TargetName,
				"address":     envVars.Address,
			})
			cancel()
			return nil
		}
	}
}

// run is the main entry point for running the service checker.
// It initializes the environment variables, sets up the checker, and starts the checking loop.
func run(ctx context.Context, getenv func(string) string, stderr io.Writer, dial DialFunc) error {
	envVars, err := parseEnv(getenv)
	if err != nil {
		return err
	}

	return runLoop(ctx, envVars, dial, stderr)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, os.Getenv, os.Stderr, defaultDialTCPContext); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
