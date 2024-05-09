/*
Copyright 2024 The Kubernetes Authors.

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
package main

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// defaultScopes:
// - cloud-platform is the base scope to authenticate to GCP.
// - userinfo.email is used to authenticate to GKE APIs with gserviceaccount email instead of numeric uniqueID.
var defaultScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

func getVMToken(ctx context.Context) (*oauth2.Token, error) {
	ts, err := google.DefaultTokenSource(ctx, defaultScopes...)
	if err != nil {
		return nil, fmt.Errorf("cannot construct google.DefaultTokenSource with scopes %v: %v", defaultScopes, err)
	}
	return ts.Token()
}
