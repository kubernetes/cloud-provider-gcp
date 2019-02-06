/*
Copyright 2019 The Kubernetes Authors.

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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
)

// altTokenSource is the structure holding the data for the functionality needed to generates tokens
type altTokenSource struct {
	oauthClient *http.Client
	tokenURL    string
	tokenBody   string
}

// Token returns a token which may be used for authentication
func (a *altTokenSource) Token() (*oauth2.Token, error) {
	req, err := http.NewRequest("POST", a.tokenURL, strings.NewReader(a.tokenBody))
	if err != nil {
		return nil, err
	}
	res, err := a.oauthClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var tok struct {
		AccessToken string    `json:"accessToken"`
		ExpireTime  time.Time `json:"expireTime"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: tok.AccessToken,
		Expiry:      tok.ExpireTime,
	}, nil
}

// newAltTokenSource constructs a new alternate token source for generating tokens.
func newAltTokenSource(tokenURL, tokenBody string) oauth2.TokenSource {
	return &altTokenSource{
		oauthClient: oauth2.NewClient(oauth2.NoContext, google.ComputeTokenSource("")),
		tokenURL:    tokenURL,
		tokenBody:   tokenBody,
	}
}
