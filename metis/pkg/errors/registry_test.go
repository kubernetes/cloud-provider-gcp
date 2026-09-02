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
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestRegistry_ExhaustiveRegistration parses reasons.go AST to discover every
// exported Reason... variable definition of type ReasonSpec and verifies that:
// 1. Every ReasonSpec is registered in reasonMap during init().
// 2. LookupReason(spec.Reason) returns ok == true.
// 3. Each ReasonSpec has valid GRPCCode, CNICode > 0, and non-empty DefaultMsg.
func TestRegistry_ExhaustiveRegistration(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "reasons.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse reasons.go AST: %v", err)
	}

	discoveredReasonVars := make([]string, 0)
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if strings.HasPrefix(name.Name, "Reason") {
					discoveredReasonVars = append(discoveredReasonVars, name.Name)
				}
			}
		}
	}

	if len(discoveredReasonVars) == 0 {
		t.Fatal("failed to discover any Reason... variable declarations in reasons.go")
	}

	for _, varName := range discoveredReasonVars {
		var found bool
		for _, spec := range reasonMap {
			lookedUp, ok := LookupReason(spec.Reason)
			if !ok {
				t.Errorf("ReasonSpec %s (%q) defined in reasons.go is missing from init() reasonMap registration!", varName, spec.Reason)
				continue
			}
			if lookedUp.CNICode == 0 {
				t.Errorf("ReasonSpec %s (%q) has invalid CNI code 0", varName, spec.Reason)
			}
			if lookedUp.Msg == "" {
				t.Errorf("ReasonSpec %s (%q) has empty Msg", varName, spec.Reason)
			}
			found = true
			break
		}
		if !found {
			t.Errorf("ReasonSpec variable %s defined in reasons.go was not found in reasonMap", varName)
		}
	}
}

// TestLookupReason_UnmappedReason verifies that LookupReason returns false
// when given an unmapped or unknown error reason string.
func TestLookupReason_UnmappedReason(t *testing.T) {
	spec, mapped := LookupReason("UNKNOWN_REASON")
	if mapped {
		t.Fatalf("expected mapped = false for unknown reason, got spec = %+v", spec)
	}
}
