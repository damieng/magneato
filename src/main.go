// Magneato by damieng - https://github.com/damieng/magneato
// main.go - Main entry point and command routing
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"fmt"
	"log"
	"os"
)

// UnpackArgs represents parsed arguments for the unpack command
type UnpackArgs struct {
	Filename   string
	OutputDir  string
	DataFormat string
}

// ParseUnpackArgs parses command line arguments for the unpack command
func ParseUnpackArgs(args []string) (UnpackArgs, error) {
    fmt.Printf("Magneato v0.1.0 - https://github.com/damieng/magneato\n")
	
	if len(args) < 1 {
		return UnpackArgs{}, fmt.Errorf("insufficient arguments")
	}
	
	filename := args[0]
	var outputDir string
	dataFormat := "binary" // default
	
	// Parse arguments
	for i := 1; i < len(args); i++ {
		if args[i] == "--data-format" {
			if i+1 >= len(args) {
				return UnpackArgs{}, fmt.Errorf("--data-format requires a value (binary, hex, or quoted)")
			}
			dataFormat = args[i+1]
			if dataFormat != "binary" && dataFormat != "hex" && dataFormat != "quoted" {
				return UnpackArgs{}, fmt.Errorf("invalid data format '%s'. Must be one of: binary, hex, quoted", dataFormat)
			}
			i++ // skip the value
		} else if outputDir == "" {
			outputDir = args[i]
		}
	}
	
	return UnpackArgs{
		Filename:   filename,
		OutputDir:  outputDir,
		DataFormat: dataFormat,
	}, nil
}

func main() {
	var command string = "magneato"
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  " + command + " info <filename.dsk>")
		fmt.Println("  " + command + " unpack <filename.dsk> [output_directory] [--data-format binary|hex|quoted]")
		fmt.Println("  " + command + " pack <unpacked_directory> <output.dsk>")
		fmt.Println("Commands:")
		fmt.Println("  info    - Display DSK file information")
		fmt.Println("  unpack  - Extract DSK to directory structure")
		fmt.Println("           (if output_directory is omitted, creates folder in current directory)")
		fmt.Println("           --data-format: binary (default), hex, or quoted (quoted-printable)")
		fmt.Println("  pack    - Reconstruct DSK from unpacked directory")
		os.Exit(1)
	}

	command = os.Args[1]

	switch command {
	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: " + command + " info <filename.dsk>")
			os.Exit(1)
		}
		filename := os.Args[2]
		fmt.Printf("dumping info for %s\n", filename)

		dsk, err := ParseDSK(filename)
		if err != nil {
			log.Fatalf("Error parsing DSK: %v", err)
		}

		dsk.DumpInfo()

	case "unpack":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run . unpack <filename.dsk> [output_directory] [--data-format binary|hex|quoted]")
			os.Exit(1)
		}
		
		unpackArgs, err := ParseUnpackArgs(os.Args[2:])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		

		fmt.Printf("unpacking %s (data-format: %s)\n", unpackArgs.Filename, unpackArgs.DataFormat)

		dsk, err := ParseDSK(unpackArgs.Filename)
		if err != nil {
			log.Fatalf("Error parsing DSK: %v", err)
		}

		if err := dsk.Unpack(unpackArgs.Filename, unpackArgs.OutputDir, unpackArgs.DataFormat); err != nil {
			log.Fatalf("Error unpacking DSK: %v", err)
		}

	case "pack":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run . pack <unpacked_directory> <output.dsk>")
			os.Exit(1)
		}
		unpackedDir := os.Args[2]
		outputFile := os.Args[3]
		fmt.Printf("packing processing %s\n", unpackedDir)
		if err := Pack(unpackedDir, outputFile); err != nil {
			log.Fatalf("Error packing DSK: %v", err)
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Commands: info, unpack, pack")
		os.Exit(1)
	}
}
