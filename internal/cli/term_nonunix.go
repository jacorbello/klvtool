//go:build !unix

package cli

import "io"

func makeRawInput(io.Reader) (func() error, error) {
	return nil, nil
}
