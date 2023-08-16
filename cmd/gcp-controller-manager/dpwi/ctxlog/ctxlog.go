/*
Copyright 2023 The Kubernetes Authors.

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

// Package ctxlog implements contextual logging.
// It adds some context before the message. Below is an example
// [event:kube-system/verified-ksa-to-gsa id:ff85d5fa]
package ctxlog

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

// contextKey is unexported to prevent collisions
type contextKey string

type fragment func(ctx context.Context) string

const (
	// EventKey is the context key for "Event"
	EventKey contextKey = "Event"
	// BackgroundIDKey is the context key for "Background"
	BackgroundIDKey contextKey = "Background"
)

var fragments = []fragment{eventFragment, backgroundIDFragment}

func eventFragment(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(EventKey)
	if v == nil {
		return ""
	}
	return fmt.Sprintf("event:%s", v)
}

func backgroundIDFragment(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(BackgroundIDKey)
	if v == nil {
		return ""
	}
	str := fmt.Sprintf("%v", v)
	if len(str) > 8 {
		str = str[:8]
	}
	return fmt.Sprintf("id:%s", str)
}

// Infof logs to the INFO log with context.
func Infof(ctx context.Context, format string, args ...interface{}) {
	klog.InfoDepth(1, addFragments(ctx, fmt.Sprintf(format, args...)))
}

// Warningf logs to the WARNING log with context.
func Warningf(ctx context.Context, format string, args ...interface{}) {
	klog.WarningDepth(1, addFragments(ctx, fmt.Sprintf(format, args...)))
}

// Errorf logs to the ERROR log with context.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	klog.ErrorDepth(1, addFragments(ctx, fmt.Sprintf(format, args...)))
}

func addFragments(ctx context.Context, ori string) string {
	var values []string
	for _, f := range fragments {
		v := f(ctx)
		if v != "" {
			values = append(values, v)
		}
	}
	return fmt.Sprintf("[%s] %s", strings.Join(values, " "), ori)
}
