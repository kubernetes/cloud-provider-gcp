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
	genproto "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/metis/api/adaptiveipam/v1"
)

// Standard ErrorInfo metadata key constants corresponding to PodInfo fields.
const (
	MetadataKeyNetwork      = "network"
	MetadataKeyPodName      = "pod_name"
	MetadataKeyPodNamespace = "pod_namespace"
	MetadataKeyContainerID  = "container_id"
)

// PodInfo holds pod metadata for error reporting and telemetry across
// Metis components.
type PodInfo struct {
	PodName      string
	PodNamespace string
	ContainerID  string
	Network      string
}

// NewPodInfoFromAllocateReq extracts PodInfo metadata from an AllocatePodIPRequest.
func NewPodInfoFromAllocateReq(req *adaptiveipam.AllocatePodIPRequest) PodInfo {
	if req == nil {
		return PodInfo{}
	}
	var containerID string
	if req.Ipv4Config != nil {
		containerID = req.Ipv4Config.ContainerId
	} else if req.Ipv6Config != nil {
		containerID = req.Ipv6Config.ContainerId
	}
	return PodInfo{
		PodName:      req.PodName,
		PodNamespace: req.PodNamespace,
		ContainerID:  containerID,
		Network:      req.Network,
	}
}

// MetisError interface implemented by all structured Metis errors.
type MetisError interface {
	error
	ReasonSpec() ReasonSpec
	GRPCCode() codes.Code
	Reason() string
	Metadata() map[string]string
	ToGRPCStatus() *status.Status
	Unwrap() error
}

// metisError represents a structured error bound to MetisErrorDomain
// ("container.googleapis.com"). It carries machine-readable error reasons,
// HTTP/gRPC codes, and underlying cause errors, converting directly into
// google.rpc.ErrorInfo gRPC status details.
type metisError struct {
	spec     ReasonSpec
	message  string
	metadata map[string]string
	cause    error
}

func (e *metisError) Error() string               { return e.message }
func (e *metisError) ReasonSpec() ReasonSpec      { return e.spec }
func (e *metisError) GRPCCode() codes.Code        { return e.spec.GRPCCode }
func (e *metisError) Reason() string              { return e.spec.Reason }
func (e *metisError) Metadata() map[string]string { return e.metadata }
func (e *metisError) Unwrap() error               { return e.cause }

func (e *metisError) ToGRPCStatus() *status.Status {
	st := status.New(e.spec.GRPCCode, e.message)
	errorInfo := &genproto.ErrorInfo{
		Reason:   e.spec.Reason,
		Domain:   MetisErrorDomain,
		Metadata: e.metadata,
	}
	stWithDetails, err := st.WithDetails(errorInfo)
	if err != nil {
		return st
	}
	return stWithDetails
}

func (e *metisError) GRPCStatus() *status.Status {
	return e.ToGRPCStatus()
}

// NewMetisError constructs a MetisError with custom parameters.
func NewMetisError(spec ReasonSpec, message string, metadata map[string]string, cause error) MetisError {
	return &metisError{
		spec:     spec,
		message:  message,
		metadata: metadata,
		cause:    cause,
	}
}

// ErrNetworkConfigInvalid indicates that request parameters or CNI netconf
// JSON parameters are invalid or missing.
func ErrNetworkConfigInvalid(msg string, metadata map[string]string, cause error) MetisError {
	if metadata == nil {
		metadata = map[string]string{}
	}
	return &metisError{
		spec:     ReasonNetworkConfigInvalid,
		message:  msg,
		metadata: metadata,
		cause:    cause,
	}
}

// ErrInternal indicates an internal server, database, or uncaught component
// error within Metis.
func ErrInternal(msg string, cause error) MetisError {
	return &metisError{
		spec:     ReasonInternalError,
		message:  msg,
		metadata: map[string]string{},
		cause:    cause,
	}
}
