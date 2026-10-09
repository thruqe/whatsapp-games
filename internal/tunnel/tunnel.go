package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"time"
)

var tunnelURLPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// StartQuickTunnel launches a cloudflared quick tunnel to expose the local server over public HTTPS.
func StartQuickTunnel(ctx context.Context, localAddr string) (string, func(), error) {
	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--no-autoupdate", "--url", localAddr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("cloudflared stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("cloudflared stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start cloudflared: %w", err)
	}

	cleanup := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	found := make(chan string, 1)

	scanStream := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if u := tunnelURLPattern.FindString(line); u != "" && u != "https://api.trycloudflare.com" {
				select {
				case found <- u:
				default:
				}
			}
		}
	}

	go scanStream(stdout)
	go scanStream(stderr)

	select {
	case u := <-found:
		log.Printf("[Tunnel] Quick tunnel established: %s -> %s", u, localAddr)
		return u, cleanup, nil
	case <-time.After(30 * time.Second):
		cleanup()
		return "", nil, errors.New("timed out waiting for cloudflared tunnel URL")
	case <-ctx.Done():
		cleanup()
		return "", nil, ctx.Err()
	}
}
