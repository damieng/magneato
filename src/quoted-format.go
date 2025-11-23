// Magneato by damieng - https://github.com/damieng/magneato
// quoted-format.go - Quoted-printable format read/write functions
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/quotedprintable"
	"os"
)

// ReadQuotedFormat reads and decodes quoted-printable data from a file
func ReadQuotedFormat(filename string) ([]byte, error) {
	encodedData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read quoted-printable file: %v", err)
	}

	reader := quotedprintable.NewReader(bytes.NewReader(encodedData))
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode quoted-printable data: %v", err)
	}

	return data, nil
}

// WriteQuotedFormat encodes binary data as quoted-printable and writes it to a file
func WriteQuotedFormat(filename string, data []byte) error {
	var buf bytes.Buffer
	writer := quotedprintable.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to encode data as quoted-printable: %v", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close quoted-printable writer: %v", err)
	}

	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write quoted-printable file: %v", err)
	}

	return nil
}

