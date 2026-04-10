package packetize

import (
	"fmt"
	"math"
)

func decodeBERLength(input []byte) (int, int, error) {
	if len(input) == 0 {
		return 0, 0, fmt.Errorf("missing BER length")
	}

	first := input[0]
	if first&0x80 == 0 {
		return int(first), 1, nil
	}

	count := int(first & 0x7f)
	if count == 0 || len(input) < 1+count {
		return 0, 0, fmt.Errorf("invalid BER length encoding")
	}

	length := 0
	for _, b := range input[1 : 1+count] {
		if length > (math.MaxInt-int(b))/256 {
			return 0, 0, fmt.Errorf("BER length overflow")
		}
		length = (length << 8) | int(b)
	}

	return length, 1 + count, nil
}
