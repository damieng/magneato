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
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// ParseDSK parses a DSK file and returns a DSK structure
func ParseDSK(filename string) (*DSK, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the whole file into memory (DSK files are small, usually < 1MB)
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)

	dsk := &DSK{}

	// 1. Parse Disk Header
	if err := binary.Read(reader, binary.LittleEndian, &dsk.Header); err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}

	// Validate Signature
	sig := string(dsk.Header.SignatureString[:])
	if !strings.HasPrefix(sig, "EXTENDED") && !strings.HasPrefix(sig, "MV - CPC") {
		return nil, fmt.Errorf("invalid file signature: %s", sig)
	}

	// 2. Parse Tracks
	// In eDSK, the TrackSizeTable indicates the size of each track block.
	// The parsing order corresponds to the file layout.

	totalBlocks := int(dsk.Header.Tracks) * int(dsk.Header.Sides)

	// Keep track of where we are in the file manually for the tracks,
	// because track sizes are variable.
	currentOffset := int64(HeaderSize)

	for i := 0; i < totalBlocks; i++ {
		// Calculate Track Size: table entry * 256
		trackSize := int(dsk.Header.TrackSizeTable[i]) * 256

		if trackSize == 0 {
			// Unformatted or empty track, skip
			continue
		}

		// Create a reader specifically for this track block
		if int64(len(data)) < currentOffset+int64(trackSize) {
			return nil, fmt.Errorf("file unexpected EOF at track %d", i)
		}

		trackData := data[currentOffset : currentOffset+int64(trackSize)]
		trackReader := bytes.NewReader(trackData)

		// Parse Track Header
		var tHeader TrackHeader
		if err := binary.Read(trackReader, binary.LittleEndian, &tHeader); err != nil {
			return nil, fmt.Errorf("failed to read track header %d: %v", i, err)
		}

		// Check Track Signature (should be "Track-Info\r\n" - 13 bytes)
		sigStr := string(bytes.TrimRight(tHeader.Signature[:], "\x00\r\n"))
		if !strings.HasPrefix(sigStr, "Track-Info") {
			// Some variants might differ, but this is standard
			fmt.Printf("Warning: Track %d signature mismatch: %s\n", i, tHeader.Signature)
		}

		logicalTrack := LogicalTrack{
			Header:  tHeader,
			Sectors: make([]LogicalSector, 0),
		}

		// Parse Sector Information List
		// The list is immediately after the track header (header is usually 0x18 bytes)
		sectorInfos := make([]SectorInfo, tHeader.SectorCount)
		for s := 0; s < int(tHeader.SectorCount); s++ {
			if err := binary.Read(trackReader, binary.LittleEndian, &sectorInfos[s]); err != nil {
				return nil, fmt.Errorf("failed to read sector info: %v", err)
			}
		}

		// Parse Sector Data
		// Sector data follows the Sector Info list.
		// Note: In extended DSK, DataLength in SectorInfo dictates size.
		// If DataLength is 0, use calculated size: 128 * 2^N.
		for _, sInfo := range sectorInfos {
			secLen := int(sInfo.DataLength)
			if secLen == 0 {
				secLen = 128 * (1 << sInfo.N)
			}

			secData := make([]byte, secLen)
			if _, err := trackReader.Read(secData); err != nil {
				// If we run out of data in the block, it might be a short dump or protection scheme
				fmt.Printf("Warning: Short read on Track %d Sector %d\n", tHeader.TrackNum, sInfo.R)
			}

			logicalTrack.Sectors = append(logicalTrack.Sectors, LogicalSector{
				Info: sInfo,
				Data: secData,
			})
		}

		dsk.Tracks = append(dsk.Tracks, logicalTrack)
		currentOffset += int64(trackSize)
	}

	return dsk, nil
}

