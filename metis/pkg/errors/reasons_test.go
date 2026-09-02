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

// TestReasons_DocCommentCoverage parses reasons.go AST to enforce that EVERY
// ReasonSpec variable definition has a doc comment explaining "Root Cause:" and
// "Expectation:".
func TestReasons_DocCommentCoverage(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "reasons.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse reasons.go AST: %v", err)
	}

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
				if !strings.HasPrefix(name.Name, "Reason") {
					continue
				}

				doc := valueSpec.Doc
				if doc == nil {
					doc = genDecl.Doc
				}
				if doc == nil {
					t.Errorf("variable %s in reasons.go has no doc comment attached!", name.Name)
					continue
				}

				text := doc.Text()
				if !strings.Contains(text, "Root Cause:") {
					t.Errorf("doc comment for variable %s in reasons.go is missing 'Root Cause:' section", name.Name)
				}
				if !strings.Contains(text, "Expectation:") {
					t.Errorf("doc comment for variable %s in reasons.go is missing 'Expectation:' section", name.Name)
				}
			}
		}
	}
}
