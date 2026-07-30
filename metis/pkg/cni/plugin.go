/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cni

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/local"
	"k8s.io/klog/v2"

	pb "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/store"
)

const defaultRPCTimeout = 10 * time.Second

type Option func(*Plugin)

// WithClientFunc sets a custom gRPC client constructor.
func WithClientFunc(fn func(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error)) Option {
	return func(p *Plugin) {
		p.newClientFunc = fn
	}
}

// WithSocketPath overrides the default daemon socket path.
func WithSocketPath(path string) Option {
	return func(p *Plugin) {
		p.socketPath = path
	}
}

// WithDBPath overrides the default SQLite database path for direct local fallback.
func WithDBPath(path string) Option {
	return func(p *Plugin) {
		p.dbPath = path
	}
}

// WithLogFile overrides the default CNI log path.
func WithLogFile(path string) Option {
	return func(p *Plugin) {
		p.logFile = path
	}
}

// NewPlugin creates a new Plugin with functional options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{
		newClientFunc: getGrpcClient,
		socketPath:    pkg.DefaultSockPath,
		dbPath:        pkg.DefaultDBPath,
		logFile:       pkg.DefaultCNILogPath,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type pluginSession struct {
	pluginConf *PluginConf
	k8sArgs    *K8sArgs
	client     pb.AdaptiveIpamClient
	conn       *grpc.ClientConn
	logger     logr.Logger
	cleanup    func()
}

func (s *pluginSession) close() {
	if s.cleanup != nil {
		s.cleanup()
	}
	if s.conn != nil {
		s.conn.Close()
	}
}

func (p *Plugin) prepare(args *skel.CmdArgs, command string) (*pluginSession, error) {
	conf, err := loadNetConf(args.StdinData)
	if err != nil {
		return nil, err
	}

	logFile := p.logFile
	if conf.LogFile != "" {
		logFile = conf.LogFile
	}

	logger, cleanup, err := p.setupLogging(args, command, logFile)
	if err != nil {
		return nil, err
	}

	var k8sArgs *K8sArgs
	if args.Args != "" {
		k8sArgs, err = loadK8sArgs(args.Args)
		if err != nil {
			cleanup()
			return nil, err
		}
	}

	socketPath := p.socketPath
	if conf.DaemonSocket != "" {
		socketPath = conf.DaemonSocket
	}

	dbPath := p.dbPath
	if conf.DBPath != "" {
		dbPath = conf.DBPath
	}

	var client pb.AdaptiveIpamClient
	var conn *grpc.ClientConn

	// Retry connecting to the daemon socket before falling back to direct mode.
	maxRetries := 3
	var clientErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		client, conn, clientErr = p.newClientFunc(socketPath)
		if clientErr == nil {
			break
		}
		if attempt < maxRetries-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	sessionCleanup := cleanup

	if clientErr != nil {
		logger.Info("Daemon unavailable, falling back to direct local IPAM engine", "socketPath", socketPath, "dbPath", dbPath, "err", clientErr)
		ctx := context.Background()
		storeInstance, err := store.NewStore(ctx, logger, dbPath)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("metis cni fallback: failed to open store at %s: %w", dbPath, err)
		}

		engine := daemon.NewIPAMEngine(logger, storeInstance, 0, store.DefaultBusyTimeout, nil)
		client = &directClientAdapter{engine: engine}

		sessionCleanup = func() {
			storeInstance.Close()
			cleanup()
		}
	}

	return &pluginSession{
		pluginConf: conf,
		k8sArgs:    k8sArgs,
		client:     client,
		conn:       conn,
		logger:     logger,
		cleanup:    sessionCleanup,
	}, nil
}

func (p *Plugin) setupLogging(args *skel.CmdArgs, command string, logFile string) (logger logr.Logger, cleanup func(), err error) {
	klog.LogToStderr(false)
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return logr.Logger{}, nil, fmt.Errorf("failed to create log directory for %s: %v", logFile, err)
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return logr.Logger{}, nil, fmt.Errorf("failed to open log file %s: %v", logFile, err)
	}
	klog.SetOutput(f)

	logger = klog.Background().WithName("metis").WithName("cni").WithValues("containerID", args.ContainerID, "command", command)
	logger.Info("Received CNI request", "netns", args.Netns, "ifName", args.IfName, "args", args.Args, "path", args.Path, "stdinData", string(args.StdinData))

	cleanup = func() {
		f.Close()
		klog.Flush()
	}

	return logger, cleanup, nil
}

func getGrpcClient(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, nil, fmt.Errorf("daemon socket file %s unavailable: %w", socketPath, err)
	}

	dialOption := grpc.WithTransportCredentials(local.NewCredentials())

	absPath, err := filepath.Abs(socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for socket %s: %v", socketPath, err)
	}
	dialTarget := fmt.Sprintf("unix://%s", absPath)

	conn, err := grpc.NewClient(dialTarget, dialOption)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to daemon: %v", err)
	}

	return pb.NewAdaptiveIpamClient(conn), conn, nil
}
