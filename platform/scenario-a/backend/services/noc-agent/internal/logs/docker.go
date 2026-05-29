// Package logs provides Docker container log collection over the Unix socket API
// without depending on the Docker SDK.
package logs

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// LogLine is a single log entry from a container.
type LogLine struct {
	Stream     string    // "stdout" or "stderr"
	Text       string
	OccurredAt time.Time
}

// DockerClient collects logs from the Docker daemon via the Unix socket.
type DockerClient struct {
	httpClient *http.Client
}

// New creates a DockerClient using the Docker Unix socket.
func New(socketPath string) *DockerClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &DockerClient{
		httpClient: &http.Client{Transport: transport, Timeout: 10 * time.Second},
	}
}

// ContainerIDByName resolves a container name to its full ID.
func (c *DockerClient) ContainerIDByName(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf("http://localhost/containers/json?filters={\"name\":[\"%s\"]}", name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("docker: list containers: %w", err)
	}
	defer resp.Body.Close()

	// Minimal JSON parse — just extract the first "Id" field
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	idx := strings.Index(s, `"Id":"`)
	if idx == -1 {
		return "", fmt.Errorf("docker: container %q not found", name)
	}
	start := idx + 6
	end := strings.Index(s[start:], `"`)
	if end == -1 {
		return "", fmt.Errorf("docker: could not parse container Id")
	}
	return s[start : start+end], nil
}

// TailLogs fetches the last n log lines from a container.
func (c *DockerClient) TailLogs(ctx context.Context, containerID string, tail int) ([]LogLine, error) {
	url := fmt.Sprintf(
		"http://localhost/containers/%s/logs?stdout=1&stderr=1&tail=%d&timestamps=1",
		containerID, tail,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: fetch logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker: logs returned %d", resp.StatusCode)
	}

	return parseMuxedStream(resp.Body)
}

// parseMuxedStream decodes Docker's multiplexed stream format.
// Each frame: [stream_type(1)][reserved(3)][size(4)][payload...]
func parseMuxedStream(r io.Reader) ([]LogLine, error) {
	var lines []LogLine
	br := bufio.NewReader(r)
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(br, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return lines, err
		}

		streamType := header[0]
		size := binary.BigEndian.Uint32(header[4:8])

		payload := make([]byte, size)
		if _, err := io.ReadFull(br, payload); err != nil {
			break
		}

		text := strings.TrimRight(string(payload), "\n\r")

		// Docker log format with timestamps: "<RFC3339nano> <message>"
		var ts time.Time
		var message string
		if len(text) > 30 && text[29] == ' ' {
			if t, err := time.Parse(time.RFC3339Nano, text[:29]); err == nil {
				ts = t
				message = text[30:]
			}
		}
		if message == "" {
			ts = time.Now()
			message = text
		}

		stream := "stdout"
		if streamType == 2 {
			stream = "stderr"
		}

		lines = append(lines, LogLine{
			Stream:     stream,
			Text:       message,
			OccurredAt: ts,
		})
	}
	return lines, nil
}
