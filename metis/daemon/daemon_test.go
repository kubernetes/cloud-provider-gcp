package daemon

import (
	"testing"
	"time"
)

func TestDaemon_Run(t *testing.T) {
	cfg := Config{
		MonitorInterval: 5 * time.Second,
		ReleaseCooldown: 1 * time.Minute,
	}

	d := NewDaemon(cfg)

	// Since Run() just logs and returns nil currently, it shouldn't error.
	err := d.Run()
	if err != nil {
		t.Fatalf("expected no error from Run(), got: %v", err)
	}
}
