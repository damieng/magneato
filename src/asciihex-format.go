// Magneato by damieng - https://github.com/damieng/magneato
// asciihex-format.go - ASCII/Hex hybrid format read/write functions
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const minRLE = 4

// ReadASCIIHexFormat reads and decodes ASCII/Hex hybrid data from a file
func ReadASCIIHexFormat(filename string) ([]byte, error) {
	encodedData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read asciihex file: %v", err)
	}

	data, err := decodeASCIIHex(string(encodedData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode asciihex data: %v", err)
	}

	return data, nil
}

// WriteASCIIHexFormat encodes binary data as ASCII/Hex hybrid and writes it to a file
func WriteASCIIHexFormat(filename string, data []byte) error {
	encodedData := []byte(encodeASCIIHex(data))

	if err := os.WriteFile(filename, encodedData, 0644); err != nil {
		return fmt.Errorf("failed to write asciihex file: %v", err)
	}

	return nil
}

func encodeASCIIHex(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	toggle := chooseToggle(data)
	var result strings.Builder
	inHex := false

	for i := 0; i < len(data); {
		runLen := countRepeats(data[i:])
		if runLen >= minRLE {
			if !inHex {
				result.WriteByte(toggle)
				inHex = true
			}
			result.WriteString(fmt.Sprintf("%02X*%X", data[i], runLen))
			i += runLen
			continue
		}

		isASCII := data[i] >= 32 && data[i] <= 126 && data[i] != toggle

		if isASCII && inHex {
			result.WriteByte(toggle)
			inHex = false
		} else if !isASCII && !inHex {
			result.WriteByte(toggle)
			inHex = true
		}

		if inHex {
			result.WriteString(fmt.Sprintf("%02X", data[i]))
		} else {
			result.WriteByte(data[i])
		}
		i++
	}

	result.WriteByte(toggle)
	return result.String()
}

func decodeASCIIHex(encoded string) ([]byte, error) {
	if len(encoded) == 0 {
		return nil, fmt.Errorf("empty string")
	}

	toggle := encoded[len(encoded)-1]
	encoded = encoded[:len(encoded)-1]

	var result bytes.Buffer
	inHex := false

	for i := 0; i < len(encoded); {
		if encoded[i] == toggle {
			inHex = !inHex
			i++
			continue
		}

		if inHex {
			if i+1 >= len(encoded) {
				return nil, fmt.Errorf("incomplete hex at position %d", i)
			}

			if i+2 < len(encoded) && encoded[i+2] == '*' {
				hexByte := encoded[i : i+2]
				val, err := strconv.ParseUint(hexByte, 16, 8)
				if err != nil {
					return nil, fmt.Errorf("invalid hex at position %d: %v", i, err)
				}

				rleEnd := i + 3
				for rleEnd < len(encoded) && isHexDigit(encoded[rleEnd]) {
					rleEnd++
				}

				countHex := encoded[i+3 : rleEnd]
				count, err := strconv.ParseInt(countHex, 16, 32)
				if err != nil {
					return nil, fmt.Errorf("invalid RLE count at position %d: %v", i, err)
				}

				for j := 0; j < int(count); j++ {
					result.WriteByte(byte(val))
				}
				i = rleEnd
			} else {
				hexByte := encoded[i : i+2]
				val, err := strconv.ParseUint(hexByte, 16, 8)
				if err != nil {
					return nil, fmt.Errorf("invalid hex at position %d: %v", i, err)
				}
				result.WriteByte(byte(val))
				i += 2
			}
		} else {
			result.WriteByte(encoded[i])
			i++
		}
	}

	return result.Bytes(), nil
}

func chooseToggle(data []byte) byte {
	freq := make(map[byte]int)
	for _, b := range data {
		if b >= 32 && b <= 126 {
			freq[b]++
		}
	}

	for b := byte(32); b <= 126; b++ {
		if freq[b] == 0 {
			return b
		}
	}

	minFreq := len(data)
	var minByte byte = '~'
	for b := byte(32); b <= 126; b++ {
		if freq[b] < minFreq {
			minFreq = freq[b]
			minByte = b
		}
	}
	return minByte
}

func countRepeats(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := 1
	for i := 1; i < len(data) && data[i] == data[0]; i++ {
		count++
	}
	return count
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
}