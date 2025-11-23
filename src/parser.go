// Magneato by damieng - https://github.com/damieng/magneato
// parser.go - DSK file parsing logic
// Dual-licensed under MIT and Apache 2.0

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
// It automatically detects whether the file is in standard or extended format
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

	if len(data) < HeaderSize {
		return nil, fmt.Errorf("file too small to contain header")
	}

	// Detect format by checking signature at offset 0-21
	sig := string(data[0:22])
	var format DSKFormat
	if strings.HasPrefix(sig, "EXTENDED") {
		format = FormatExtended
	} else if strings.HasPrefix(sig, "MV - CPC") {
		format = FormatStandard
	} else {
		return nil, fmt.Errorf("invalid file signature: %s", sig)
	}

	// Route to appropriate parser
	if format == FormatStandard {
		return parseStandardDSK(data)
	}
	return parseExtendedDSK(data)
}

// parseExtendedDSK parses an Extended DSK file
func parseExtendedDSK(data []byte) (*DSK, error) {
	reader := bytes.NewReader(data)

	dsk := &DSK{
		Format: FormatExtended,
	}

	// 1. Parse Disk Header
	if err := binary.Read(reader, binary.LittleEndian, &dsk.Header); err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
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

// parseStandardDSK parses a Standard DSK file
func parseStandardDSK(data []byte) (*DSK, error) {
	reader := bytes.NewReader(data)

	dsk := &DSK{
		Format: FormatStandard,
	}

	// 1. Parse Disk Header (same structure, but different interpretation)
	if err := binary.Read(reader, binary.LittleEndian, &dsk.Header); err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}

	// For standard format, read the fixed track size from offset 32-33
	// This is stored in the padding field of DiskHeader
	dsk.StandardTrackSize = binary.LittleEndian.Uint16(data[32:34])
	
	// Validate header values
	if dsk.Header.Tracks == 0 || dsk.Header.Tracks > 85 {
		return nil, fmt.Errorf("invalid number of tracks: %d (must be 1-85)", dsk.Header.Tracks)
	}
	if dsk.Header.Sides == 0 || dsk.Header.Sides > 2 {
		return nil, fmt.Errorf("invalid number of sides: %d (must be 1-2)", dsk.Header.Sides)
	}
	
	// Validate track size
	if dsk.StandardTrackSize < 0x100 {
		return nil, fmt.Errorf("invalid track size: %d (must be at least 0x100 to include track info block)", dsk.StandardTrackSize)
	}
	
	// Calculate expected file size and validate
	expectedFileSize := int64(HeaderSize) + int64(dsk.Header.Tracks)*int64(dsk.Header.Sides)*int64(dsk.StandardTrackSize)
	if int64(len(data)) < expectedFileSize {
		return nil, fmt.Errorf("file too small: have %d bytes, need at least %d bytes for %d tracks x %d sides x %d bytes/track", 
			len(data), expectedFileSize, dsk.Header.Tracks, dsk.Header.Sides, dsk.StandardTrackSize)
	}

	// 2. Parse Tracks
	// In standard DSK, all tracks have the same size
	totalBlocks := int(dsk.Header.Tracks) * int(dsk.Header.Sides)
	trackSize := int(dsk.StandardTrackSize)

	// Track data starts at offset 0x100
	currentOffset := int64(HeaderSize)

	for i := 0; i < totalBlocks; i++ {
		// Check if we have enough data
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
		// Be lenient - allow null bytes after \r\n
		expectedSig := []byte("Track-Info\r\n")
		sigValid := len(tHeader.Signature) >= 13 && bytes.Equal(tHeader.Signature[:13], expectedSig)
		
		// If signature is invalid, this might be an unformatted track
		// In standard format, all tracks exist but may be unformatted (filled with 0xE5 or similar)
		if !sigValid {
			// Check if it looks like an unformatted track (all same byte value like 0xE5)
			isUnformatted := true
			if len(trackData) > 0 {
				firstByte := trackData[0]
				for j := 1; j < len(trackData) && j < 100; j++ {
					if trackData[j] != firstByte {
						isUnformatted = false
						break
					}
				}
			}
			
			if isUnformatted {
				// Unformatted track - create empty track with 0 sectors
				logicalTrack := LogicalTrack{
					Header: TrackHeader{
						TrackNum:    uint8(i / int(dsk.Header.Sides)),
						SideNum:     uint8(i % int(dsk.Header.Sides)),
						SectorCount: 0,
					},
					Sectors: make([]LogicalSector, 0),
				}
				dsk.Tracks = append(dsk.Tracks, logicalTrack)
				currentOffset += int64(trackSize)
				continue
			}
			
			// Not unformatted but signature mismatch - show warning but continue
			sigStr := string(bytes.TrimRight(tHeader.Signature[:], "\x00\r\n"))
			fmt.Printf("Warning: Track %d signature mismatch. Expected 'Track-Info\\r\\n', got: %q (hex: %x)\n", i, sigStr, tHeader.Signature[:13])
		}

		logicalTrack := LogicalTrack{
			Header:  tHeader,
			Sectors: make([]LogicalSector, 0),
		}

		// Validate sector count is reasonable
		if tHeader.SectorCount > 64 {
			return nil, fmt.Errorf("track %d has invalid sector count: %d", i, tHeader.SectorCount)
		}

		// If no sectors, skip to next track
		if tHeader.SectorCount == 0 {
			dsk.Tracks = append(dsk.Tracks, logicalTrack)
			currentOffset += int64(trackSize)
			continue
		}

		// Parse Sector Information List
		// In standard format, sector info is only 6 bytes (not 8 like extended format)
		// Layout: C, H, R, N, FDCStatus1, FDCStatus2 (bytes 06-07 are unused/0)
		// The sector info list starts immediately after the track header (offset 0x18)
		sectorInfos := make([]SectorInfo, tHeader.SectorCount)
		for s := 0; s < int(tHeader.SectorCount); s++ {
			// Read 6 bytes for standard format sector info
			sectorInfoBytes := make([]byte, 6)
			if _, err := trackReader.Read(sectorInfoBytes); err != nil {
				return nil, fmt.Errorf("failed to read sector info: %v", err)
			}
			// Parse into SectorInfo struct
			sectorInfos[s] = SectorInfo{
				C:          sectorInfoBytes[0],
				H:          sectorInfoBytes[1],
				R:          sectorInfoBytes[2],
				N:          sectorInfoBytes[3],
				FDCStatus1: sectorInfoBytes[4],
				FDCStatus2: sectorInfoBytes[5],
				DataLength: 0, // Not used in standard format
			}
		}

		// Sector data starts at offset 0x100 from the start of the track block
		// This is a fixed offset regardless of how many sector info entries there are
		// There may be padding/gap between the sector info list and sector data
		sectorDataOffset := int64(0x100)
		if int64(len(trackData)) < sectorDataOffset {
			return nil, fmt.Errorf("track %d too small for sector data (size: %d, need offset %d)", i, len(trackData), sectorDataOffset)
		}

		// Create a new reader starting at the sector data
		trackReader = bytes.NewReader(trackData[sectorDataOffset:])

		// Parse Sector Data
		// In standard format, sector size is determined by the track header's SectorSize parameter
		// All sectors in a track must be the same size (the maximum size if they differ)
		// Sector size calculation: 128 * 2^N
		// The track header's SectorSize should match the maximum N from sector info entries
		// For 8k sectors (N=6), only 1800h (6144) bytes is stored
		
		// Validate SectorSize is reasonable (typically 0-6, max 7 for 16KB)
		if tHeader.SectorSize > 7 {
			return nil, fmt.Errorf("track %d has invalid sector size N=%d (must be 0-7)", i, tHeader.SectorSize)
		}
		
		secLen := 128 * (1 << tHeader.SectorSize)
		// Special case: For 8k sectors (N=6), only 1800h bytes is stored
		if tHeader.SectorSize == 6 {
			secLen = 0x1800
		}
		
		// Additional validation: sector size should be reasonable
		if secLen > 16384 {
			return nil, fmt.Errorf("track %d calculated sector size too large: %d bytes (N=%d)", i, secLen, tHeader.SectorSize)
		}

		// Calculate remaining space for sector data
		// Sector data starts at 0x100, track size includes the 0x100 byte track info block
		remainingDataSize := int64(trackSize) - 0x100
		if remainingDataSize < 0 {
			return nil, fmt.Errorf("track %d size %d is too small (must be at least 0x100)", i, trackSize)
		}

		for _, sInfo := range sectorInfos {
			// Check if we have enough data remaining
			if int64(len(trackData))-sectorDataOffset < int64(secLen) {
				return nil, fmt.Errorf("track %d sector %d: not enough data (need %d bytes, have %d)", 
					tHeader.TrackNum, sInfo.R, secLen, int64(len(trackData))-sectorDataOffset)
			}

			secData := make([]byte, secLen)
			n, err := trackReader.Read(secData)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read sector data for track %d sector %d: %v", tHeader.TrackNum, sInfo.R, err)
			}
			if n < secLen {
				// Partial read - pad with zeros or report warning
				fmt.Printf("Warning: Short read on Track %d Sector %d: read %d of %d bytes\n", tHeader.TrackNum, sInfo.R, n, secLen)
				// Zero out the rest
				for j := n; j < secLen; j++ {
					secData[j] = 0
				}
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

// GetTrack returns a pointer to a LogicalTrack if found (by cylinder and head)
func (d *DSK) GetTrack(cylinder int, head int) *LogicalTrack {
	for i := range d.Tracks {
		if int(d.Tracks[i].Header.TrackNum) == cylinder && int(d.Tracks[i].Header.SideNum) == head {
			return &d.Tracks[i]
		}
	}
	return nil
}
