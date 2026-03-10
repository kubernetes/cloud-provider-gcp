package main

import (
	"time"

	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"k8s.io/metis/daemon"
)

const (
	defaultLogFile = "/var/log/gke-ipam-cni.log"
)

func main() {
	var cfg daemon.Config

	var daemonMode bool
	var logFile string

	// Define command-line flags to configure the daemon
	pflag.DurationVar(&cfg.MonitorInterval, "monitor-interval", 5*time.Second, "Monitor interval (e.g., 5s, 1m)")
	pflag.DurationVar(&cfg.ReleaseCooldown, "release-cooldown", 1*time.Minute, "Release cooldown duration (e.g., 5m)")
	pflag.BoolVar(&daemonMode, "daemon", false, "Run the binary in daemon mode")
	pflag.StringVar(&logFile, "daemon-log-file", defaultLogFile, "Log file for daemon")

	cliflag.InitFlags()

	logs.InitLogs()
	defer logs.FlushLogs()

	if daemonMode {
		d := daemon.NewDaemon(cfg)
		if err := d.Run(); err != nil {
			klog.Fatalf("Error: %v", err)
		}
	} else {
		klog.Info("Metis started (not in daemon mode).")
	}
}
