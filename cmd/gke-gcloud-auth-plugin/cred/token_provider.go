package cred

import (
	"time"
)

type tokenProvider interface {
	// getToken returns a new token, and the expiry of that token
	getToken() (token string, expiry *time.Time, err error)
	// useCache returns whether or not tokens from this providers should be cached
	useCache() bool
}
