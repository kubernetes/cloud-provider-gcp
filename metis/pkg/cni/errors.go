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
	"errors"
	"fmt"

	"github.com/containernetworking/cni/pkg/types"
	genproto "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	metiserrors "k8s.io/metis/pkg/errors"
)

// ToCNIError converts an error into a CNI specification-compliant
// *types.Error.
func ToCNIError(err error, fallbackMsg string) *types.Error {
	if err == nil {
		return nil
	}

	// If err is already a CNI spec error, return it directly to ensure idempotency
	// and preserve pre-formed CNI errors (e.g. config parsing failures).
	if cniErr, ok := errors.AsType[*types.Error](err); ok {
		return cniErr
	}

	st, ok := status.FromError(err)
	if !ok {
		return types.NewError(types.ErrInternal, fallbackMsg, err.Error())
	}

	cniCode := types.ErrInternal
	msg := fallbackMsg
	details := st.Message()

	for _, detail := range st.Details() {
		if errorInfo, ok := detail.(*genproto.ErrorInfo); ok && errorInfo.Domain == metiserrors.MetisErrorDomain {
			if spec, mapped := metiserrors.LookupReason(errorInfo.Reason); mapped {
				cniCode = spec.CNICode
				msg = spec.Msg
			}
			details = fmt.Sprintf("Domain: %s; Reason: %s; Message: %s", errorInfo.Domain, errorInfo.Reason, st.Message())
			if len(errorInfo.Metadata) > 0 {
				details = fmt.Sprintf("%s; Metadata: %v", details, errorInfo.Metadata)
			}
			break
		}
	}

	return types.NewError(cniCode, msg, details)
}
