// Magneato by damieng - https://github.com/damieng/magneato
// pack.go - Pack command implementation
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
	
	// Set default signature based on format (if available)
	// Default to extended format signature
	sigBytes := []byte("EXTENDED CPC DSK File\r\nDisk-Info\r\n")
	if formatStr, ok := diskMeta["format"].(string); ok && formatStr == "standard" {
		// Standard format signature (must start with "MV - CPC" and be 34 bytes)
		sigBytes = make([]byte, 34)
		copy(sigBytes, "MV - CPCEMU Disk-File")
		// Rest is padded with zeros
	}
	// Ensure signature is exactly 34 bytes
	copy(header.SignatureString[:], sigBytes)
	for i := len(sigBytes); i < len(header.SignatureString); i++ {
		header.SignatureString[i] = 0
	}
	
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
		
		// Signature is fixed: "Track-Info\r\n" (13 bytes)
		sigBytes := []byte("Track-Info\r\n")
		copy(trackHeader.Signature[:], sigBytes)
		// Pad with zeros if needed
		for j := len(sigBytes); j < len(trackHeader.Signature); j++ {
			trackHeader.Signature[j] = 0
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
				if err = json.Unmarshal(sectorMetaJSON, &sectorMeta); err != nil {
					return fmt.Errorf("failed to parse sector metadata: %v", err)
				}
				
				sectorInfo := SectorInfo{}
				cylinder, _ := sectorMeta["cylinder"].(float64)
				sectorInfo.C = uint8(cylinder)
				head, _ := sectorMeta["head"].(float64)
				sectorInfo.H = uint8(head)
				sectorID, _ := sectorMeta["sector_id"].(float64)
				sectorInfo.R = uint8(sectorID)
				sectorSize, _ := sectorMeta["sector_size"].(float64)
				sectorInfo.N = uint8(sectorSize)
				fdcStatus1, _ := sectorMeta["fdc_status1"].(float64)
				sectorInfo.FDCStatus1 = uint8(fdcStatus1)
				fdcStatus2, _ := sectorMeta["fdc_status2"].(float64)
				sectorInfo.FDCStatus2 = uint8(fdcStatus2)
				dataLength, _ := sectorMeta["data_length"].(float64)
				sectorInfo.DataLength = uint16(dataLength)
				
				sectorInfos = append(sectorInfos, sectorInfo)
				
				// Detect format and get file path
				dataFormat, sectorDataPath, err := DetectFormatFromFile(trackDir, sectorNum)
				if err != nil {
					return fmt.Errorf("failed to detect format for sector %d in track %d: %v", sectorNum, i, err)
				}
				
				// Get the appropriate reader function and read sector data
				reader, err := GetFormatReader(dataFormat)
				if err != nil {
					return fmt.Errorf("failed to get format reader for sector %d: %v", sectorNum, err)
				}
				
				sectorData, err := reader(sectorDataPath)
				if err != nil {
					return fmt.Errorf("failed to read sector data for sector %d: %v", sectorNum, err)
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

