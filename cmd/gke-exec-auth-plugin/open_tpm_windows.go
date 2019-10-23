package main

import (
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

// The path argument is ignored on Windows.
func openTPM(path string) (*realTPM, error) {
	rw, err := tpm2.OpenTPM()
	if err != nil {
		return nil, fmt.Errorf("tpm2.OpenTPM(): %v", err)
	}
	return &realTPM{rw}, nil
}
