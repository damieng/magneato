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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GetTrack returns a pointer to a LogicalTrack if found (by cylinder and head)
func (d *DSK) GetTrack(cylinder int, head int) *LogicalTrack {
	for i := range d.Tracks {
		if int(d.Tracks[i].Header.TrackNum) == cylinder && int(d.Tracks[i].Header.SideNum) == head {
			return &d.Tracks[i]
		}
	}
	return nil
}

// DumpInfo prints the DSK structure to console
func (d *DSK) DumpInfo() {
	fmt.Println("==================================================")
	fmt.Println("              eDSK FILE INFORMATION               ")
	fmt.Println("==================================================")
	fmt.Printf("Signature : %s\n", string(bytes.Trim(d.Header.SignatureString[:], "\x00")))
	fmt.Printf("Creator   : %s\n", string(bytes.Trim(d.Header.CreatorString[:], "\x00")))
	fmt.Printf("Tracks    : %d\n", d.Header.Tracks)
	fmt.Printf("Sides     : %d\n", d.Header.Sides)
	fmt.Println("--------------------------------------------------")

	for i, t := range d.Tracks {
		fmt.Printf("LogTrack #%02d | Cyl: %02d | Head: %d | SecCount: %02d | Gap3: %02d\n",
			i, t.Header.TrackNum, t.Header.SideNum, t.Header.SectorCount, t.Header.Gap3Length)

		for _, s := range t.Sectors {
			dataPreview := ""
			if len(s.Data) > 16 {
				dataPreview = hex.EncodeToString(s.Data[:16]) + "..."
			} else {
				dataPreview = hex.EncodeToString(s.Data)
			}

			fmt.Printf("   [SEC] ID: %02X | N: %d (%d bytes) | ST1: %02X ST2: %02X | Data: %s\n",
				s.Info.R, s.Info.N, len(s.Data), s.Info.FDCStatus1, s.Info.FDCStatus2, dataPreview)
		}
		fmt.Println("- - - - - - - - - - - - - - - - - - - - - - - - -")
	}
}

// Unpack extracts the DSK image to a directory structure
// If outputDir is empty, creates a folder matching the DSK filename (minus extension) in the current directory
// If outputDir is specified, creates the folder there
func (d *DSK) Unpack(dskFilename string, outputDir string) error {
	// Get base name without extension
	baseName := strings.TrimSuffix(filepath.Base(dskFilename), filepath.Ext(dskFilename))
	
	// Determine root directory
	var rootDir string
	if outputDir != "" {
		// Use specified output directory, creating the base name folder inside it
		rootDir = filepath.Join(outputDir, baseName)
	} else {
		// Use current behavior: create folder in current directory
		rootDir = baseName
	}
	
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory: %v", err)
	}

	// Create disk-image.meta
	// Convert TrackSizeTable to slice of integers for JSON (not []uint8 which gets base64 encoded)
	trackSizeTableSlice := make([]int, len(d.Header.TrackSizeTable))
	for i, v := range d.Header.TrackSizeTable {
		trackSizeTableSlice[i] = int(v)
	}
	
	diskMeta := map[string]interface{}{
		"signature":       string(bytes.Trim(d.Header.SignatureString[:], "\x00")),
		"creator":         string(bytes.Trim(d.Header.CreatorString[:], "\x00")),
		"tracks":          d.Header.Tracks,
		"sides":           d.Header.Sides,
		"track_size_table": trackSizeTableSlice,
	}

	diskMetaPath := filepath.Join(rootDir, "disk-image.meta")
	diskMetaJSON, err := json.MarshalIndent(diskMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal disk metadata: %v", err)
	}
	if err := os.WriteFile(diskMetaPath, diskMetaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write disk metadata: %v", err)
	}

	// Create a map to quickly find tracks by their position index
	trackMap := make(map[int]*LogicalTrack)
	for i := range d.Tracks {
		// Calculate position index: track_number * sides + side_number
		posIdx := int(d.Tracks[i].Header.TrackNum)*int(d.Header.Sides) + int(d.Tracks[i].Header.SideNum)
		trackMap[posIdx] = &d.Tracks[i]
	}

	// Process all possible track positions (including unformatted ones)
	totalBlocks := int(d.Header.Tracks) * int(d.Header.Sides)
	for i := 0; i < totalBlocks; i++ {
		// Calculate track number and side from position index
		trackNum := i / int(d.Header.Sides)
		sideNum := i % int(d.Header.Sides)
		
		// Check if this track is formatted (exists in trackMap)
		track, hasTrack := trackMap[i]
		
		// Create track directory (format: track-XX-side-Y or track-XX)
		trackDirName := fmt.Sprintf("track-%02d", i)
		if d.Header.Sides > 1 {
			trackDirName = fmt.Sprintf("track-%02d-side-%d", trackNum, sideNum)
		}
		trackDir := filepath.Join(rootDir, trackDirName)

		if err := os.MkdirAll(trackDir, 0755); err != nil {
			return fmt.Errorf("failed to create track directory: %v", err)
		}

		// Create track.meta
		var trackMeta map[string]interface{}
		if hasTrack && track != nil {
			// Formatted track - use actual track header data
			// Convert byte arrays to slices for JSON
			signatureSlice := make([]uint8, len(track.Header.Signature))
			copy(signatureSlice, track.Header.Signature[:])
			unusedSlice := make([]uint8, len(track.Header.Unused))
			copy(unusedSlice, track.Header.Unused[:])
			unused2Slice := make([]uint8, len(track.Header.Unused2))
			copy(unused2Slice, track.Header.Unused2[:])
			
			trackMeta = map[string]interface{}{
				"signature":    signatureSlice,
				"unused":       unusedSlice,
				"track_number": track.Header.TrackNum,
				"side_number":  track.Header.SideNum,
				"unused2":      unused2Slice,
				"sector_size":  track.Header.SectorSize,
				"sector_count": track.Header.SectorCount,
				"gap3_length":  track.Header.Gap3Length,
				"filler_byte":  track.Header.FillerByte,
				"formatted":    true,
			}
		} else {
			// Unformatted track - create minimal metadata
			defaultSignature := []byte("Track-Info\r\n")
			signatureSlice := make([]uint8, 13)
			copy(signatureSlice, defaultSignature)
			
			trackMeta = map[string]interface{}{
				"signature":    signatureSlice,
				"unused":       []uint8{0, 0, 0}, // 3 bytes per spec (not 4)
				"track_number": uint8(trackNum),
				"side_number":  uint8(sideNum),
				"unused2":      []uint8{0, 0},
				"sector_size":  uint8(0),
				"sector_count": uint8(0),
				"gap3_length":  uint8(0),
				"filler_byte":  uint8(0),
				"formatted":    false,
			}
		}

		trackMetaPath := filepath.Join(trackDir, "track.meta")
		trackMetaJSON, err := json.MarshalIndent(trackMeta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal track metadata: %v", err)
		}
		if err := os.WriteFile(trackMetaPath, trackMetaJSON, 0644); err != nil {
			return fmt.Errorf("failed to write track metadata: %v", err)
		}

		// Process sectors only if track is formatted
		if hasTrack && track != nil {
			for _, sector := range track.Sectors {
				sectorNum := sector.Info.R

				// Create sector-n.data
				sectorDataPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.data", sectorNum))
				if err := os.WriteFile(sectorDataPath, sector.Data, 0644); err != nil {
					return fmt.Errorf("failed to write sector data: %v", err)
				}

				// Create sector-n.meta
				sectorMeta := map[string]interface{}{
					"cylinder":    sector.Info.C,
					"head":        sector.Info.H,
					"sector_id":   sector.Info.R,
					"sector_size": sector.Info.N,
					"fdc_status1": sector.Info.FDCStatus1,
					"fdc_status2": sector.Info.FDCStatus2,
					"data_length": sector.Info.DataLength,
				}

				sectorMetaPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.meta", sectorNum))
				sectorMetaJSON, err := json.MarshalIndent(sectorMeta, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal sector metadata: %v", err)
				}
				if err := os.WriteFile(sectorMetaPath, sectorMetaJSON, 0644); err != nil {
					return fmt.Errorf("failed to write sector metadata: %v", err)
				}
			}
		}
	}

	fmt.Printf("Successfully unpacked DSK to: %s\n", rootDir)
	return nil
}

// Pack reconstructs a DSK file from an unpacked directory structure
func Pack(unpackedDir string, outputFilename string) error {
	// Read disk-image.meta
	diskMetaPath := filepath.Join(unpackedDir, "disk-image.meta")
	diskMetaJSON, err := os.ReadFile(diskMetaPath)
	if err != nil {
		return fmt.Errorf("failed to read disk metadata: %v", err)
	}

	var diskMeta map[string]interface{}
	if err := json.Unmarshal(diskMetaJSON, &diskMeta); err != nil {
		return fmt.Errorf("failed to parse disk metadata: %v", err)
	}

	// Reconstruct DiskHeader
	header := DiskHeader{}
	
	// Signature
	sigStr, ok := diskMeta["signature"].(string)
	if !ok {
		return fmt.Errorf("invalid signature in disk metadata")
	}
	copy(header.SignatureString[:], []byte(sigStr))
	
	// Creator
	creatorStr, ok := diskMeta["creator"].(string)
	if !ok {
		return fmt.Errorf("invalid creator in disk metadata")
	}
	copy(header.CreatorString[:], []byte(creatorStr))
	
	// Tracks and Sides
	tracksFloat, ok := diskMeta["tracks"].(float64)
	if !ok {
		return fmt.Errorf("invalid tracks in disk metadata")
	}
	header.Tracks = uint8(tracksFloat)
	
	sidesFloat, ok := diskMeta["sides"].(float64)
	if !ok {
		return fmt.Errorf("invalid sides in disk metadata")
	}
	header.Sides = uint8(sidesFloat)
	
	// TrackSizeTable - CRITICAL for reconstruction
	trackSizeTableInterface, ok := diskMeta["track_size_table"]
	if !ok {
		return fmt.Errorf("missing track_size_table in disk metadata - cannot reconstruct file")
	}
	
	// Handle different JSON unmarshaling types
	var trackSizeTableValues []uint8
	switch v := trackSizeTableInterface.(type) {
	case []interface{}:
		// JSON unmarshaled as []interface{}
		trackSizeTableValues = make([]uint8, len(v))
		for i, val := range v {
			switch num := val.(type) {
			case float64:
				trackSizeTableValues[i] = uint8(num)
			case uint8:
				trackSizeTableValues[i] = num
			case int:
				trackSizeTableValues[i] = uint8(num)
			default:
				return fmt.Errorf("invalid track_size_table entry type at index %d: %T", i, val)
			}
		}
	case []float64:
		// JSON unmarshaled as []float64
		trackSizeTableValues = make([]uint8, len(v))
		for i, val := range v {
			trackSizeTableValues[i] = uint8(val)
		}
	case []uint8:
		// JSON unmarshaled as []uint8 (unlikely but possible)
		trackSizeTableValues = v
	default:
		return fmt.Errorf("invalid track_size_table format in disk metadata: expected array, got %T", trackSizeTableInterface)
	}
	
	if len(trackSizeTableValues) > len(header.TrackSizeTable) {
		return fmt.Errorf("track_size_table too large: %d > %d", len(trackSizeTableValues), len(header.TrackSizeTable))
	}
	copy(header.TrackSizeTable[:], trackSizeTableValues)

	// Create output file
	outFile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Write header manually to ensure correct structure
	// Write signature (34 bytes)
	if _, err := outFile.Write(header.SignatureString[:]); err != nil {
		return fmt.Errorf("failed to write signature: %v", err)
	}
	
	// Write creator (14 bytes)
	if _, err := outFile.Write(header.CreatorString[:]); err != nil {
		return fmt.Errorf("failed to write creator: %v", err)
	}
	
	// Write tracks (1 byte)
	if _, err := outFile.Write([]byte{header.Tracks}); err != nil {
		return fmt.Errorf("failed to write tracks: %v", err)
	}
	
	// Write sides (1 byte)
	if _, err := outFile.Write([]byte{header.Sides}); err != nil {
		return fmt.Errorf("failed to write sides: %v", err)
	}
	
	// Write 2 bytes of padding (unused)
	if _, err := outFile.Write([]byte{0, 0}); err != nil {
		return fmt.Errorf("failed to write padding: %v", err)
	}
	
	// Write track size table - only write tracks * sides entries
	trackTableSize := int(header.Tracks) * int(header.Sides)
	if trackTableSize > len(header.TrackSizeTable) {
		return fmt.Errorf("track table size %d exceeds maximum %d", trackTableSize, len(header.TrackSizeTable))
	}
	if _, err := outFile.Write(header.TrackSizeTable[:trackTableSize]); err != nil {
		return fmt.Errorf("failed to write track size table: %v", err)
	}
	
	// Pad the rest of the header to 256 bytes (0x100)
	// We've written: 34 + 14 + 1 + 1 + 2 + trackTableSize = 52 + trackTableSize bytes
	// Need to pad to 256 bytes
	bytesWritten := 52 + trackTableSize
	if bytesWritten < HeaderSize {
		padding := make([]byte, HeaderSize-bytesWritten)
		if _, err := outFile.Write(padding); err != nil {
			return fmt.Errorf("failed to write header padding: %v", err)
		}
	}

	// Process tracks in order (based on TrackSizeTable)
	totalBlocks := int(header.Tracks) * int(header.Sides)
	
	for i := 0; i < totalBlocks; i++ {
		trackSize := int(header.TrackSizeTable[i]) * 256
		
		// Calculate track number and side from position index
		trackNum := i / int(header.Sides)
		sideNum := i % int(header.Sides)
		
		// Find track directory
		// Try both naming conventions
		trackDirName := fmt.Sprintf("track-%02d", i)
		trackDir := filepath.Join(unpackedDir, trackDirName)
		
		// If not found, try the side-specific naming
		if _, err := os.Stat(trackDir); os.IsNotExist(err) {
			if header.Sides > 1 {
				trackDirName = fmt.Sprintf("track-%02d-side-%d", trackNum, sideNum)
				trackDir = filepath.Join(unpackedDir, trackDirName)
			}
		}
		
		// Check if track directory exists
		if _, err := os.Stat(trackDir); os.IsNotExist(err) {
			// Track directory doesn't exist - this means it's unformatted
			if trackSize == 0 {
				// Expected - unformatted track, skip
				continue
			} else {
				// Unexpected - track should exist but doesn't
				return fmt.Errorf("track %d (track %d, side %d) should exist but directory not found", i, trackNum, sideNum)
			}
		}
		
		// If trackSize is 0, this is an unformatted track - skip writing data
		if trackSize == 0 {
			// Unformatted track - skip writing track data
			continue
		}

		// Read track.meta
		trackMetaPath := filepath.Join(trackDir, "track.meta")
		trackMetaJSON, err := os.ReadFile(trackMetaPath)
		if err != nil {
			return fmt.Errorf("failed to read track metadata for track %d: %v", i, err)
		}

		var trackMeta map[string]interface{}
		if err := json.Unmarshal(trackMetaJSON, &trackMeta); err != nil {
			return fmt.Errorf("failed to parse track metadata for track %d: %v", i, err)
		}

		// Reconstruct TrackHeader
		trackHeader := TrackHeader{}
		
		// Signature (13 bytes: "Track-Info\r\n")
		sigArray, ok := trackMeta["signature"].([]interface{})
		if !ok {
			// Default signature if missing (13 bytes)
			sigBytes := []byte("Track-Info\r\n")
			copy(trackHeader.Signature[:], sigBytes)
			// Pad with zeros if needed
			for i := len(sigBytes); i < len(trackHeader.Signature); i++ {
				trackHeader.Signature[i] = 0
			}
		} else {
			for j, v := range sigArray {
				if j >= len(trackHeader.Signature) {
					break
				}
				val, _ := v.(float64)
				trackHeader.Signature[j] = uint8(val)
			}
		}
		
		// Unused
		unusedArray, ok := trackMeta["unused"].([]interface{})
		if ok {
			for j, v := range unusedArray {
				if j >= len(trackHeader.Unused) {
					break
				}
				val, _ := v.(float64)
				trackHeader.Unused[j] = uint8(val)
			}
		}
		
		trackNumMeta, _ := trackMeta["track_number"].(float64)
		trackHeader.TrackNum = uint8(trackNumMeta)
		
		sideNumMeta, _ := trackMeta["side_number"].(float64)
		trackHeader.SideNum = uint8(sideNumMeta)
		
		// Unused2
		unused2Array, ok := trackMeta["unused2"].([]interface{})
		if ok {
			for j, v := range unused2Array {
				if j >= len(trackHeader.Unused2) {
					break
				}
				val, _ := v.(float64)
				trackHeader.Unused2[j] = uint8(val)
			}
		}
		
		sectorSize, _ := trackMeta["sector_size"].(float64)
		trackHeader.SectorSize = uint8(sectorSize)
		
		sectorCount, _ := trackMeta["sector_count"].(float64)
		trackHeader.SectorCount = uint8(sectorCount)
		
		gap3Length, _ := trackMeta["gap3_length"].(float64)
		trackHeader.Gap3Length = uint8(gap3Length)
		
		fillerByte, _ := trackMeta["filler_byte"].(float64)
		trackHeader.FillerByte = uint8(fillerByte)

		// Write track header
		if err := binary.Write(outFile, binary.LittleEndian, &trackHeader); err != nil {
			return fmt.Errorf("failed to write track header %d: %v", i, err)
		}

		// Read and write sectors
		// Read sector files in order
		sectorInfos := make([]SectorInfo, 0, trackHeader.SectorCount)
		sectorDataMap := make(map[uint8][]byte)
		
		// Read all sector files
		entries, err := os.ReadDir(trackDir)
		if err != nil {
			return fmt.Errorf("failed to read track directory: %v", err)
		}
		
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "sector-") && strings.HasSuffix(entry.Name(), ".meta") {
				sectorNumStr := strings.TrimPrefix(strings.TrimSuffix(entry.Name(), ".meta"), "sector-")
				var sectorNum uint8
				if _, err := fmt.Sscanf(sectorNumStr, "%d", &sectorNum); err != nil {
					continue
				}
				
				// Read sector metadata
				sectorMetaPath := filepath.Join(trackDir, entry.Name())
				sectorMetaJSON, err := os.ReadFile(sectorMetaPath)
				if err != nil {
					return fmt.Errorf("failed to read sector metadata: %v", err)
				}
				
				var sectorMeta map[string]interface{}
				if err := json.Unmarshal(sectorMetaJSON, &sectorMeta); err != nil {
					return fmt.Errorf("failed to parse sector metadata: %v", err)
				}
				
				sectorInfo := SectorInfo{}
				cylinder, _ := sectorMeta["cylinder"].(float64)
				sectorInfo.C = uint8(cylinder)
				head, _ := sectorMeta["head"].(float64)
				sectorInfo.H = uint8(head)
				sectorId, _ := sectorMeta["sector_id"].(float64)
				sectorInfo.R = uint8(sectorId)
				sectorSize, _ := sectorMeta["sector_size"].(float64)
				sectorInfo.N = uint8(sectorSize)
				fdcStatus1, _ := sectorMeta["fdc_status1"].(float64)
				sectorInfo.FDCStatus1 = uint8(fdcStatus1)
				fdcStatus2, _ := sectorMeta["fdc_status2"].(float64)
				sectorInfo.FDCStatus2 = uint8(fdcStatus2)
				dataLength, _ := sectorMeta["data_length"].(float64)
				sectorInfo.DataLength = uint16(dataLength)
				
				sectorInfos = append(sectorInfos, sectorInfo)
				
				// Read sector data
				sectorDataPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.data", sectorNum))
				sectorData, err := os.ReadFile(sectorDataPath)
				if err != nil {
					return fmt.Errorf("failed to read sector data: %v", err)
				}
				sectorDataMap[sectorNum] = sectorData
			}
		}
		
		// Sort sectors by sector ID (R field) to maintain order
		// Use a simple insertion sort or just write in the order they were found
		// For now, we'll write them in the order they appear in the directory
		// which should match the original order if unpack preserved it
		
		// Write sector info list
		for _, sectorInfo := range sectorInfos {
			if err := binary.Write(outFile, binary.LittleEndian, &sectorInfo); err != nil {
				return fmt.Errorf("failed to write sector info: %v", err)
			}
		}
		
		// Write sector data in the same order
		for _, sectorInfo := range sectorInfos {
			sectorData := sectorDataMap[sectorInfo.R]
			if _, err := outFile.Write(sectorData); err != nil {
				return fmt.Errorf("failed to write sector data: %v", err)
			}
		}
		
		// Pad track to the expected size if necessary
		currentPos, _ := outFile.Seek(0, io.SeekCurrent)
		trackStartPos := int64(HeaderSize)
		for j := 0; j < i; j++ {
			trackStartPos += int64(header.TrackSizeTable[j]) * 256
		}
		bytesWritten := currentPos - trackStartPos
		if bytesWritten < int64(trackSize) {
			padding := make([]byte, int64(trackSize)-bytesWritten)
			// Fill with filler byte if specified, otherwise zeros
			filler := trackHeader.FillerByte
			for j := range padding {
				padding[j] = filler
			}
			if _, err := outFile.Write(padding); err != nil {
				return fmt.Errorf("failed to pad track: %v", err)
			}
		}
	}

	fmt.Printf("Successfully packed DSK to: %s\n", outputFilename)
	return nil
}

