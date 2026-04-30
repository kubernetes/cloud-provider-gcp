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
	"flag"
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
		logFile:       pkg.DefaultCNILogPath,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type pluginSession struct {
	conf    *NetConf
	k8sArgs *K8sArgs
	client  pb.AdaptiveIpamClient
	conn    *grpc.ClientConn
	logger  logr.Logger
	cleanup func()
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

	client, conn, err := p.newClientFunc(socketPath)
	if err != nil {
		cleanup()
		return nil, err
	}

	return &pluginSession{
		conf:    conf,
		k8sArgs: k8sArgs,
		client:  client,
		conn:    conn,
		logger:  logger,
		cleanup: cleanup,
	}, nil
}

func (p *Plugin) setupLogging(args *skel.CmdArgs, command string, logFile string) (logger logr.Logger, cleanup func(), err error) {
	// We must explicitly set these klog flags to false, otherwise klog
	// ignores the SetOutput file writer and dumps all logs to stderr.
	_ = flag.Set("logtostderr", "false")
	_ = flag.Set("alsologtostderr", "false")

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
