package radio

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestProbeResolution(t *testing.T) {
	videoID := "BGXOYfZMR0w" // Tycho VOD
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r := New()
	tr, err := r.Resolve(ctx, videoID, true)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	t.Logf("Resolved URL (IsLive=%v): %s", tr.IsLive, tr.URL)

	// HTTP Probe
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(tr.URL)
	if err != nil {
		t.Fatalf("HTTP Head error: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("HTTP Status: %s", resp.Status)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got: %s", resp.Status)
	}
}

