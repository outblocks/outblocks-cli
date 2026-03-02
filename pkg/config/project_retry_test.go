package config

import (
	"context"
	"errors"
	"testing"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

func TestDownloadPluginWithRetryRetriesAndSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	want := &plugins.Plugin{}

	got, err := downloadPluginWithRetry(context.Background(), 3, 0, func() (*plugins.Plugin, error) {
		attempts++

		if attempts < 3 {
			return nil, errors.New("temporary")
		}

		return want, nil
	})
	if err != nil {
		t.Fatalf("downloadPluginWithRetry returned error: %v", err)
	}

	if got != want {
		t.Fatalf("unexpected plugin pointer: got %p want %p", got, want)
	}

	if attempts != 3 {
		t.Fatalf("unexpected attempts count: got %d want %d", attempts, 3)
	}
}

func TestDownloadPluginWithRetryReturnsLastError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("still failing")
	attempts := 0

	_, err := downloadPluginWithRetry(context.Background(), 3, 0, func() (*plugins.Plugin, error) {
		attempts++

		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}

	if attempts != 3 {
		t.Fatalf("unexpected attempts count: got %d want %d", attempts, 3)
	}
}
