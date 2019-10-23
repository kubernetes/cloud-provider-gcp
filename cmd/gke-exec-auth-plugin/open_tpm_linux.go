// +build !windows

package main

import (
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

func openTPM(path string) (*realTPM, error) {
	rw, err := tpm2.OpenTPM(path)
	if err != nil {
		return nil, fmt.Errorf("tpm2.OpenTPM(%q): %v", path, err)
	}
	return &realTPM{rw}, nil
}
