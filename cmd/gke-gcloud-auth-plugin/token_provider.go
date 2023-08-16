package main

import (
	"time"
)

type tokenProvider interface {
	// token returns a new token, and the expiry of that token
	token() (token string, expiry *time.Time, err error)
	// useCache returns whether or not tokens from this providers should be cached
	useCache() bool
	// extraArgs returns all of the extra arguments passed along when getting the token
	getExtraArgs() []string
	// getGcloudArgs command args gets all of the arguments passed to gcloud for getting the token
	getGcloudArgs() []string
}
