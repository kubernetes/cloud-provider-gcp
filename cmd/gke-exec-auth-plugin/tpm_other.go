// +build !windows

package main

import (
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

func openTPM() (*realTPM, error) {
	rw, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		return nil, fmt.Errorf("tpm2.OpenTPM(%q): %v", *tpmPath, err)
	}
	return &realTPM{rw}, nil
}
