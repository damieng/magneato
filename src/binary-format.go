// Magneato by damieng - https://github.com/damieng/magneato
// binary-format.go - Binary format read/write functions
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"fmt"
	"os"
)

// ReadBinaryFormat reads binary data from a file
func ReadBinaryFormat(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read binary file: %v", err)
	}

	return data, nil
}

// WriteBinaryFormat writes binary data to a file
func WriteBinaryFormat(filename string, data []byte) error {
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write binary file: %v", err)
	}

	return nil
}

