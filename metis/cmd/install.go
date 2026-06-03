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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// newInstallCommand returns a command to install the CNI plugin.
// The daemon image uses a distroless base image that lacks a shell, making a
// standard shell script CNI installer unusable.
func newInstallCommand() *cobra.Command {
	opts := newInstallOptions()

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install metis CNI binary to the host CNI path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			return runInstallWithOptions(opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

// runInstallWithOptions performs the self-installation sequence of the metis CNI
// binary onto the host CNI path.
func runInstallWithOptions(opts *installOptions) error {
	// Locate and open the currently executing binary.
	sourcePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve currently executing binary path: %w", err)
	}
	klog.Infof("Resolved current executable path: %q", sourcePath)

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source binary %q: %w", sourcePath, err)
	}

	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source binary %q: %w", sourcePath, err)
	}
	defer src.Close()

	// Resolve the installation paths based on target CNI directory.
	destDir := filepath.Join(opts.CniDir, "bin")
	binaryName := filepath.Base(sourcePath)
	destPath := filepath.Join(destDir, binaryName)

	// Ensure the target host directory structure exists.
	klog.Infof("Creating destination directory %q...", destDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %q: %w", destDir, err)
	}

	// Stage the copy to a temporary file in the target directory. This prevents
	// partial-write corruption if the copy is interrupted (e.g. due to OOM,
	// disk exhaustion, or eviction) and prepares for a zero-downtime atomic swap.
	destTempPath := destPath + ".tmp"
	_ = os.Remove(destTempPath)

	dst, err := os.OpenFile(destTempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to open destination temp file %q: %w", destTempPath, err)
	}
	// Clean up the temporary file if the swap sequence fails or is aborted.
	defer func() {
		dst.Close()
		_ = os.Remove(destTempPath)
	}()

	// Stream binary bytes from source to the staged temporary file.
	klog.Infof("Copying binary bytes from %q to %q...", sourcePath, destTempPath)
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy binary bytes to %q: %w", destTempPath, err)
	}

	// Force filesystem cache commit to guarantee all writes reach physical disk.
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination temp file %q: %w", destTempPath, err)
	}

	// Close the file explicitly before rename to avoid ETXTBSY (Text file busy)
	// if Kubelet executes it immediately. It is safe to call Close() twice
	// (the deferred Close() will run when returning, returning an ignorable error).
	if err := dst.Close(); err != nil {
		return fmt.Errorf("failed to close destination temp file %q: %w", destTempPath, err)
	}

	// Set executable permissions on the temp binary BEFORE swapping it into place.
	// This ensures the CNI plugin is immediately usable by Kubelet the exact
	// microsecond it appears at the target path, avoiding any unexecutable windows.
	if err := os.Chmod(destTempPath, sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to set permissions on temp file %q: %w", destTempPath, err)
	}

	// Execute an atomic swap via rename(2). In Linux, rename(2) is an atomic
	// directory metadata operation. If the CNI is actively executing, rename(2)
	// successfully unlinks the old inode without throwing ETXTBSY (Text file busy).
	// The running process continues executing the old memory pages, while any
	// new Kubelet invocations instantly route to the newly installed binary.
	klog.Infof("Installing metis CNI binary to %q...", destPath)
	if err := os.Rename(destTempPath, destPath); err != nil {
		return fmt.Errorf("failed to rename %q to %q: %w", destTempPath, destPath, err)
	}

	klog.Infof("Metis CNI binary installed successfully to %q", destPath)
	return nil
}
