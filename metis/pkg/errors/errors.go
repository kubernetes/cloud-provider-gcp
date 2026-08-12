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

package errors

import (
	"fmt"

	genproto "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const MetisErrorDomain = "metis.google.com"

// Well-defined error reasons across Metis
const (
	ReasonIPPoolExhausted = "IP_POOL_EXHAUSTED"
)

// MetisError interface implemented by all internal domain errors.
type MetisError interface {
	error
	GRPCCode() codes.Code
	Reason() string
	Metadata() map[string]string
	ToGRPCStatus() *status.Status
	Unwrap() error
}

type domainError struct {
	code     codes.Code
	reason   string
	message  string
	metadata map[string]string
	cause    error
}

func (e *domainError) Error() string               { return e.message }
func (e *domainError) GRPCCode() codes.Code        { return e.code }
func (e *domainError) Reason() string              { return e.reason }
func (e *domainError) Metadata() map[string]string { return e.metadata }
func (e *domainError) Unwrap() error               { return e.cause }

func (e *domainError) ToGRPCStatus() *status.Status {
	st := status.New(e.code, e.message)
	errorInfo := &genproto.ErrorInfo{
		Reason:   e.reason,
		Domain:   MetisErrorDomain,
		Metadata: e.metadata,
	}
	stWithDetails, err := st.WithDetails(errorInfo)
	if err != nil {
		return st
	}
	return stWithDetails
}

// NewDomainError constructs a MetisError with custom parameters.
func NewDomainError(code codes.Code, reason, message string, metadata map[string]string, cause error) MetisError {
	return &domainError{
		code:     code,
		reason:   reason,
		message:  message,
		metadata: metadata,
		cause:    cause,
	}
}

// Factory Constructors for Developers

// ErrIPPoolExhausted indicates that the local node IP address pool is exhausted.
func ErrIPPoolExhausted(network string, cause error) MetisError {
	return &domainError{
		code:     codes.ResourceExhausted,
		reason:   ReasonIPPoolExhausted,
		message:  fmt.Sprintf("IP address pool exhausted for network %s", network),
		metadata: map[string]string{"network": network},
		cause:    cause,
	}
}
