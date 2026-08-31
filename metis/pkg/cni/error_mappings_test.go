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
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	metiserrors "k8s.io/metis/pkg/errors"
)

// TestCNIMappings_ExhaustiveInvariants dynamically parses pkg/errors/reasons.go
// AST to discover every exported Reason... constant and enforces that EVERY reason
// defined in reasons.go is registered in reasonToCNIMap with a valid positive CNI
// code and non-empty default message.
func TestCNIMappings_ExhaustiveInvariants(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "../errors/reasons.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse ../errors/reasons.go AST: %v", err)
	}

	var discoveredReasons []string
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if strings.HasPrefix(name.Name, "Reason") {
					discoveredReasons = append(discoveredReasons, name.Name)
				}
			}
		}
	}

	if len(discoveredReasons) == 0 {
		t.Fatal("failed to discover any Reason... constants in ../errors/reasons.go")
	}

	// Known mappings value map for discovered Reason identifiers
	reasonValueMap := map[string]string{
		"ReasonNetworkConfigInvalid": metiserrors.ReasonNetworkConfigInvalid,
		"ReasonInternalError":        metiserrors.ReasonInternalError,
	}

	for _, reasonIdent := range discoveredReasons {
		reasonVal, known := reasonValueMap[reasonIdent]
		if !known {
			t.Errorf("new Reason constant %q discovered in reasons.go but not added to reasonValueMap check in error_mappings_test.go", reasonIdent)
			continue
		}
		mapping, ok := reasonToCNIMap[reasonVal]
		if !ok {
			t.Errorf("pkg/errors Reason %q (%s) is missing from reasonToCNIMap in cni/error_mappings.go!", reasonIdent, reasonVal)
			continue
		}
		if mapping.code == 0 {
			t.Errorf("reason %q mapped to invalid CNI code 0", reasonVal)
		}
		if mapping.defaultMsg == "" {
			t.Errorf("reason %q has empty default message", reasonVal)
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
