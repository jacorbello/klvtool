package model

import (
	"fmt"
	"testing"
)

func TestTSErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		code Code
	}{
		{"TSSync", TSSync(fmt.Errorf("bad sync")), CodeTSSync},
		{"TSParse", TSParse(fmt.Errorf("bad header")), CodeTSParse},
		{"TSRead", TSRead(fmt.Errorf("io error")), CodeTSRead},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.code)
			}
		})
	}
}
