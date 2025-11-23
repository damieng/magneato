// Magneato by damieng - https://github.com/damieng/magneato
// main_test.go - Unit tests for command line parsing
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"strings"
	"testing"
)

func TestParseUnpackArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expected    UnpackArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal args - just filename",
			args: []string{"unpack", "test.dsk"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "binary",
			},
			expectError: false,
		},
		{
			name: "filename with output directory",
			args: []string{"unpack", "test.dsk", "output"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "output",
				DataFormat: "binary",
			},
			expectError: false,
		},
		{
			name: "filename with data format binary",
			args: []string{"unpack", "test.dsk", "--data-format", "binary"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "binary",
			},
			expectError: false,
		},
		{
			name: "filename with data format hex",
			args: []string{"unpack", "test.dsk", "--data-format", "hex"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "hex",
			},
			expectError: false,
		},
		{
			name: "filename with data format quoted",
			args: []string{"unpack", "test.dsk", "--data-format", "quoted"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "quoted",
			},
			expectError: false,
		},
		{
			name: "filename, output dir, and data format",
			args: []string{"unpack", "test.dsk", "output", "--data-format", "hex"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "output",
				DataFormat: "hex",
			},
			expectError: false,
		},
		{
			name: "filename, data format, and output dir (order swapped)",
			args: []string{"unpack", "test.dsk", "--data-format", "quoted", "output"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "output",
				DataFormat: "quoted",
			},
			expectError: false,
		},
		{
			name: "insufficient arguments",
			args: []string{"unpack"},
			expected: UnpackArgs{
				Filename:   "",
				OutputDir:  "",
				DataFormat: "binary",
			},
			expectError: true,
			errorMsg:    "insufficient arguments",
		},
		{
			name: "data format missing value",
			args: []string{"unpack", "test.dsk", "--data-format"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "binary",
			},
			expectError: true,
			errorMsg:    "--data-format requires a value",
		},
		{
			name: "invalid data format",
			args: []string{"unpack", "test.dsk", "--data-format", "invalid"},
			expected: UnpackArgs{
				Filename:   "test.dsk",
				OutputDir:  "",
				DataFormat: "binary",
			},
			expectError: true,
			errorMsg:    "invalid data format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseUnpackArgs(tt.args)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result.Filename != tt.expected.Filename {
					t.Errorf("Filename: expected %q, got %q", tt.expected.Filename, result.Filename)
				}
				if result.OutputDir != tt.expected.OutputDir {
					t.Errorf("OutputDir: expected %q, got %q", tt.expected.OutputDir, result.OutputDir)
				}
				if result.DataFormat != tt.expected.DataFormat {
					t.Errorf("DataFormat: expected %q, got %q", tt.expected.DataFormat, result.DataFormat)
				}
			}
		})
	}
}

