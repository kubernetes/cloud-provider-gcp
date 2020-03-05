package healthz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	passingCheck := func(context.Context) error { return nil }
	failingCheck := func(context.Context) error { return errors.New("failed") }
	blockedCheck := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	tests := []struct {
		desc       string
		checks     map[string]Check
		wantStatus int
	}{
		{
			desc:       "no checks",
			wantStatus: http.StatusOK,
		},
		{
			desc:       "one passing check",
			wantStatus: http.StatusOK,
			checks: map[string]Check{
				"passing": passingCheck,
			},
		},
		{
			desc:       "multiple passing checks",
			wantStatus: http.StatusOK,
			checks: map[string]Check{
				"passing 1": passingCheck,
				"passing 2": passingCheck,
				"passing 3": passingCheck,
			},
		},
		{
			desc:       "one failing check",
			wantStatus: http.StatusInternalServerError,
			checks: map[string]Check{
				"failing": failingCheck,
			},
		},
		{
			desc:       "multiple failing checks",
			wantStatus: http.StatusInternalServerError,
			checks: map[string]Check{
				"failing 1": failingCheck,
				"failing 2": failingCheck,
			},
		},
		{
			desc:       "passing and failing checks",
			wantStatus: http.StatusInternalServerError,
			checks: map[string]Check{
				"passing 1": passingCheck,
				"failing":   failingCheck,
				"passing 2": passingCheck,
			},
		},
		{
			desc:       "timeout",
			wantStatus: http.StatusInternalServerError,
			checks: map[string]Check{
				"blocked": blockedCheck,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			h := NewHandler()
			h.Checks = tt.checks

			s := httptest.NewServer(h)
			defer s.Close()

			resp, err := http.Get(s.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %q, want %q", http.StatusText(resp.StatusCode), http.StatusText(tt.wantStatus))
			}
		})
	}
}
