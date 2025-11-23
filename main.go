// Copyright (c) 2024 Magneato Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Alternatively, this file may be used under the terms of the MIT license:
//
// Copyright (c) 2024 Magneato Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
		fmt.Println("  go run . unpack <filename.dsk> [output_directory]")
		fmt.Println("  go run . pack <unpacked_directory> <output.dsk>")
		fmt.Println("Commands:")
		fmt.Println("  info    - Display DSK file information")
		fmt.Println("  unpack  - Extract DSK to directory structure")
		fmt.Println("           (if output_directory is omitted, creates folder in current directory)")
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
			fmt.Println("Usage: go run . unpack <filename.dsk> [output_directory]")
			os.Exit(1)
		}
		filename := os.Args[2]
		var outputDir string
		if len(os.Args) >= 4 {
			outputDir = os.Args[3]
		}
		fmt.Printf("Processing file: %s\n", filename)
		if outputDir != "" {
			fmt.Printf("Output directory: %s\n", outputDir)
		}

		dsk, err := ParseDSK(filename)
		if err != nil {
			log.Fatalf("Error parsing DSK: %v", err)
		}

		if err := dsk.Unpack(filename, outputDir); err != nil {
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

