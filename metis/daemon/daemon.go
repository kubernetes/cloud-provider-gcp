package daemon

import (
	"time"

	"k8s.io/klog/v2"
)

// Config contains the configuration parameters for the daemon.
type Config struct {
	MonitorInterval time.Duration
	ReleaseCooldown time.Duration
}

// Daemon represents the metis daemon process.
type Daemon struct {
	Config Config
}

// NewDaemon creates a new Daemon instance with the given configuration.
func NewDaemon(cfg Config) *Daemon {
	return &Daemon{
		Config: cfg,
	}
}

// Run starts the daemon process.
func (d *Daemon) Run() error {
	klog.Infof("metis daemon has started successfully with config: %+v", d.Config)
	return nil
}
