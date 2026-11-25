package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// healthcheckTimeout bounds a single `issuetrax healthcheck` probe.
const healthcheckTimeout = 3 * time.Second

// healthcheck probes the running server's /healthz endpoint. It exists so the
// distroless image, which has no shell or curl, can still declare a container
// HEALTHCHECK. The address comes from ISSUETRAX_ADDR (default :8080); a
// wildcard or empty host is probed on loopback.
func healthcheck() error {
	addr := os.Getenv("ISSUETRAX_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse ISSUETRAX_ADDR %q: %w", addr, err)
	}

	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	url := "http://" + net.JoinHostPort(host, port) + "/healthz"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // the operator sets ISSUETRAX_ADDR; only that host is probed
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // see above
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s: status %d", url, resp.StatusCode)
	}

	return nil
}
