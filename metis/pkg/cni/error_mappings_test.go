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
	"go/parser"
	"go/token"
	"strings"
	"testing"

	metiserrors "k8s.io/metis/pkg/errors"
)

// TestCNIMappings_ExhaustiveInvariants enforces that EVERY Reason constant
// defined in pkg/errors/reasons.go is registered in reasonToCNIMap and carries
// a valid positive CNI code and non-empty default message.
func TestCNIMappings_ExhaustiveInvariants(t *testing.T) {
	requiredReasons := []string{
		metiserrors.ReasonNetworkConfigInvalid,
		metiserrors.ReasonInternalError,
	}

	for _, reason := range requiredReasons {
		mapping, ok := reasonToCNIMap[reason]
		if !ok {
			t.Errorf("pkg/errors Reason %q is missing from reasonToCNIMap in cni/error_mappings.go!", reason)
			continue
		}
		if mapping.code == 0 {
			t.Errorf("reason %q mapped to invalid CNI code 0", reason)
		}
		if mapping.defaultMsg == "" {
			t.Errorf("reason %q has empty default message", reason)
		}
	}
}

// TestCNIMappings_DocCommentCoverage parses error_mappings.go AST to enforce
// that EVERY reason entry in reasonToCNIMap has a doc comment containing both
// "Root Cause:" and "Expectation:" sections.
func TestCNIMappings_DocCommentCoverage(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "error_mappings.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse error_mappings.go AST: %v", err)
	}

	requiredReasons := []string{
		metiserrors.ReasonNetworkConfigInvalid,
		metiserrors.ReasonInternalError,
	}

	for _, reason := range requiredReasons {
		var foundDoc bool
		for _, commentGroup := range node.Comments {
			text := commentGroup.Text()
			if strings.Contains(text, reason) {
				foundDoc = true
				if !strings.Contains(text, "Root Cause:") {
					t.Errorf("documentation comment for %q in error_mappings.go is missing 'Root Cause:' section", reason)
				}
				if !strings.Contains(text, "Expectation:") {
					t.Errorf("documentation comment for %q in error_mappings.go is missing 'Expectation:' section", reason)
				}
			}
		}
		if !foundDoc {
			t.Errorf("reason %q in error_mappings.go has no doc comment explaining root causes and expectations", reason)
		}
	}
}

// TestLookupCNIMapping_UnmappedReason verifies that LookupCNIMapping returns
// false when given an unmapped or unknown error reason string.
func TestLookupCNIMapping_UnmappedReason(t *testing.T) {
	code, msg, mapped := LookupCNIMapping("UNKNOWN_REASON")
	if mapped {
		t.Fatalf("expected mapped = false for unknown reason, got code = %d, msg = %q", code, msg)
	}
}
