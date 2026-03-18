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

package kops

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// HydrateTemplate reads the template file and performs variable substitution.
func HydrateTemplate(c *Config) error {
	var content []byte
	var err error

	if c.TemplateSrc != "" {
		content, err = os.ReadFile(c.TemplateSrc)
		if err != nil {
			fmt.Printf("Warning: failed to read template source %s: %v. Using default template.\n", c.TemplateSrc, err)
			content = []byte(defaultTemplate)
		}
	} else {
		content = []byte(defaultTemplate)
	}

	// ONLY variables that were hydrated by envsubst in the original bash script.
	// This ensures that kOps-internal placeholders (like {{ .clusterName }}) are preserved.
	vars := map[string]string{
		"K8S_VERSION":                c.K8sVersion,
		"GCP_PROJECT":                c.GCPProject,
		"GCP_LOCATION":               c.GCPLocation,
		"GCP_ZONES":                  c.GCPZones,
		"CONTROL_PLANE_MACHINE_TYPE": c.ControlPlaneMachineType,
		"NODE_MACHINE_TYPE":          c.NodeMachineType,
		"NODE_COUNT":                 strconv.Itoa(c.NodeCount),
		"NEW_CCM_SPEC":               c.NewCCMSpec,
	}

	hydrated := os.Expand(string(content), func(name string) string {
		if val, ok := vars[name]; ok {
			return val
		}
		// If not in our specific list, return the original string (e.g. $GOPATH)
		return "$" + name
	})

	// Ensure workdir exists
	if err := os.MkdirAll(filepath.Dir(c.TemplatePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for hydrated template: %v", err)
	}

	if err := os.WriteFile(c.TemplatePath, []byte(hydrated), 0644); err != nil {
		return fmt.Errorf("failed to write hydrated template: %v", err)
	}

	fmt.Printf("Hydrated template written to %s\n", c.TemplatePath)
	return nil
}
