// Magneato by damieng - https://github.com/damieng/magneato
// main.go - Main entry point and command routing
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  go run . info <filename.dsk>")
		fmt.Println("  go run . unpack <filename.dsk> [output_directory] [--data-format binary|hex|quoted]")
		fmt.Println("  go run . pack <unpacked_directory> <output.dsk>")
		fmt.Println("Commands:")
		fmt.Println("  info    - Display DSK file information")
		fmt.Println("  unpack  - Extract DSK to directory structure")
		fmt.Println("           (if output_directory is omitted, creates folder in current directory)")
		fmt.Println("           --data-format: binary (default), hex, or quoted (quoted-printable)")
		fmt.Println("  pack    - Reconstruct DSK from unpacked directory")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run . info <filename.dsk>")
			os.Exit(1)
		}
		filename := os.Args[2]
		fmt.Printf("Processing file: %s\n", filename)

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
		filename := os.Args[2]
		var outputDir string
		dataFormat := "binary" // default
		
		// Parse arguments
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "--data-format" {
				if i+1 >= len(os.Args) {
					fmt.Println("Error: --data-format requires a value (binary, hex, or quoted)")
					os.Exit(1)
				}
				dataFormat = os.Args[i+1]
				if dataFormat != "binary" && dataFormat != "hex" && dataFormat != "quoted" {
					fmt.Printf("Error: invalid data format '%s'. Must be one of: binary, hex, quoted\n", dataFormat)
					os.Exit(1)
				}
				i++ // skip the value
			} else if outputDir == "" {
				outputDir = os.Args[i]
			}
		}
		
		fmt.Printf("Processing file: %s\n", filename)
		if outputDir != "" {
			fmt.Printf("Output directory: %s\n", outputDir)
		}
		fmt.Printf("Data format: %s\n", dataFormat)

		dsk, err := ParseDSK(filename)
		if err != nil {
			log.Fatalf("Error parsing DSK: %v", err)
		}

		if err := dsk.Unpack(filename, outputDir, dataFormat); err != nil {
			log.Fatalf("Error unpacking DSK: %v", err)
		}
	case "pack":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run . pack <unpacked_directory> <output.dsk>")
			os.Exit(1)
		}
		unpackedDir := os.Args[2]
		outputFile := os.Args[3]
		fmt.Printf("Packing directory: %s\n", unpackedDir)
		if err := Pack(unpackedDir, outputFile); err != nil {
			log.Fatalf("Error packing DSK: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Commands: info, unpack, pack")
		os.Exit(1)
	}
}
