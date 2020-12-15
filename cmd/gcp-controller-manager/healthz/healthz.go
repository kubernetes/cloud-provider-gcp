package healthz

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// Check reports the (un)healthiness of a single component.
type Check func(context.Context) error

// Handler is a http.Handler that performs a number of named checks and returns
// their result on every request.
// Note: populate the Checks field *before* accepting any requests to the
// Handler.
type Handler struct {
	// Timeout is passed to Check calls via the context. It limits how long
	// Handler allows Checks to run for.
	Timeout time.Duration
	// Checks are named Check functions for Handler to call on each request.
	// Checks are called sequentially, in random order.
	Checks map[string]Check
}

// NewHandler initializes a new Handler.
func NewHandler() *Handler {
	return &Handler{
		Timeout: time.Second,
		Checks:  make(map[string]Check),
	}
}

// ServeHTTP executes all checks in h and writes the result to rw. If any check
// failed, response code will be 500 (Internal Server Error), otherwise 200
// (OK).
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), h.Timeout)
	defer cancel()

	var errs []string
	for name, check := range h.Checks {
		if err := check(ctx); err != nil {
			klog.Warningf("healthz check %q failed: %v", name, err)
			errs = append(errs, fmt.Sprintf("%q: %v", name, err))
		}
	}
	if len(errs) > 0 {
		http.Error(rw, strings.Join(errs, "\n"), http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(rw, "ok")
}
