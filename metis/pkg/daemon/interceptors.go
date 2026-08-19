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

package daemon

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metiserrors "k8s.io/metis/pkg/errors"
)

// ErrorInterceptor intercepts unary gRPC requests and formats MetisError into google.rpc.ErrorInfo status.
func (s *adaptiveIpamServer) ErrorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err == nil {
		return resp, nil
	}

	if mErr, ok := errors.AsType[metiserrors.MetisError](err); ok {
		s.logger.V(2).Info("gRPC request failed with Metis error",
			"method", info.FullMethod,
			"reason", mErr.Reason(),
			"message", mErr.Error(),
			"cause", mErr.Unwrap())
		return nil, mErr.ToGRPCStatus().Err()
	}

	s.logger.Error(err, "Programmer error: handler returned non-MetisError", "method", info.FullMethod)
	return nil, status.Error(codes.Internal, err.Error())
}
