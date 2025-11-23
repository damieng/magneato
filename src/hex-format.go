// Magneato by damieng - https://github.com/damieng/magneato
// hex-format.go - Hexadecimal format read/write functions
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"encoding/hex"
	"fmt"
	"os"
)

// ReadHexFormat reads and decodes hexadecimal data from a file
func ReadHexFormat(filename string) ([]byte, error) {
	encodedData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read hex file: %v", err)
	}

	data, err := hex.DecodeString(string(encodedData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex data: %v", err)
	}

	return data, nil
}

// WriteHexFormat encodes binary data as hexadecimal and writes it to a file
func WriteHexFormat(filename string, data []byte) error {
	encodedData := []byte(hex.EncodeToString(data))

	if err := os.WriteFile(filename, encodedData, 0644); err != nil {
		return fmt.Errorf("failed to write hex file: %v", err)
	}

	return nil
}

