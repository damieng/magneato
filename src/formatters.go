// Magneato by damieng - https://github.com/damieng/magneato
// formatters.go - Format read/write functions and formatter selection
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"mime/quotedprintable"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const minRLE = 4

// Binary Format

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

// Hex Format

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

// Quoted Format

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

// ASCIIHex Format

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

// Formatter Selection

// FormatReader is a function type for reading formatted data
type FormatReader func(string) ([]byte, error)

// FormatWriter is a function type for writing formatted data
type FormatWriter func(string, []byte) error

// GetFormatReader returns the appropriate reader function for a given format name
func GetFormatReader(format string) (FormatReader, error) {
	switch format {
	case "binary":
		return ReadBinaryFormat, nil
	case "hex":
		return ReadHexFormat, nil
	case "quoted":
		return ReadQuotedFormat, nil
	case "asciihex":
		return ReadASCIIHexFormat, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// GetFormatWriter returns the appropriate writer function for a given format name
func GetFormatWriter(format string) (FormatWriter, error) {
	switch format {
	case "binary":
		return WriteBinaryFormat, nil
	case "hex":
		return WriteHexFormat, nil
	case "quoted":
		return WriteQuotedFormat, nil
	case "asciihex":
		return WriteASCIIHexFormat, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// DetectFormatFromFile detects the format of a sector data file by checking which files exist
// Returns the format name, file path, and error
func DetectFormatFromFile(trackDir string, sectorNum uint8) (string, string, error) {
	binPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.bin", sectorNum))
	hexPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.hex", sectorNum))
	quotedPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.quoted", sectorNum))
	asciihexPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.asciihex", sectorNum))

	var existingFiles []string
	var sectorDataPath string
	var dataFormat string

	if _, err := os.Stat(binPath); err == nil {
		existingFiles = append(existingFiles, "binary")
		if sectorDataPath == "" {
			sectorDataPath = binPath
			dataFormat = "binary"
		}
	}
	if _, err := os.Stat(hexPath); err == nil {
		existingFiles = append(existingFiles, "hex")
		if sectorDataPath == "" {
			sectorDataPath = hexPath
			dataFormat = "hex"
		}
	}
	if _, err := os.Stat(quotedPath); err == nil {
		existingFiles = append(existingFiles, "quoted")
		if sectorDataPath == "" {
			sectorDataPath = quotedPath
			dataFormat = "quoted"
		}
	}
	if _, err := os.Stat(asciihexPath); err == nil {
		existingFiles = append(existingFiles, "asciihex")
		if sectorDataPath == "" {
			sectorDataPath = asciihexPath
			dataFormat = "asciihex"
		}
	}

	if len(existingFiles) == 0 {
		return "", "", fmt.Errorf("no sector data file found for sector %d (expected sector-%d.bin, sector-%d.hex, sector-%d.quoted, or sector-%d.asciihex)", sectorNum, sectorNum, sectorNum, sectorNum, sectorNum)
	}

	if len(existingFiles) > 1 {
		return "", "", fmt.Errorf("multiple sector data files found for sector %d: %v (only one format should exist)", sectorNum, existingFiles)
	}

	return dataFormat, sectorDataPath, nil
}

