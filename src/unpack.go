// Magneato by damieng - https://github.com/damieng/magneato
// unpack.go - Unpack command and encoding/decoding utilities
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Unpack extracts the DSK image to a directory structure
// If outputDir is empty, creates a folder matching the DSK filename (minus extension) in the current directory
// If outputDir is specified, creates the folder there
// dataFormat can be "binary", "hex", "quoted" (quoted-printable), or "asciihex"
func (d *DSK) Unpack(dskFilename string, outputDir string, dataFormat string) error {
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
	diskMeta := map[string]interface{}{
		"creator": string(bytes.Trim(d.Header.CreatorString[:], "\x00")),
		"tracks":  d.Header.Tracks,
		"sides":   d.Header.Sides,
		"format":  "extended",
	}

	// Add format-specific metadata
	if d.Format == FormatStandard {
		diskMeta["format"] = "standard"
		diskMeta["track_size"] = d.StandardTrackSize
	} else {
		// Convert TrackSizeTable to slice of integers for JSON (not []uint8 which gets base64 encoded)
		trackSizeTableSlice := make([]int, len(d.Header.TrackSizeTable))
		for i, v := range d.Header.TrackSizeTable {
			trackSizeTableSlice[i] = int(v)
		}
		diskMeta["track_size_table"] = trackSizeTableSlice
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
			unusedSlice := make([]uint8, len(track.Header.Unused))
			copy(unusedSlice, track.Header.Unused[:])
			unused2Slice := make([]uint8, len(track.Header.Unused2))
			copy(unused2Slice, track.Header.Unused2[:])
			
			trackMeta = map[string]interface{}{
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
			trackMeta = map[string]interface{}{
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

				// Get the appropriate writer function and determine file path
				writer, err := GetFormatWriter(dataFormat)
				if err != nil {
					return fmt.Errorf("failed to get format writer: %v", err)
				}
				
				// Determine file extension based on format
				var ext string
				switch dataFormat {
				case "hex":
					ext = "hex"
				case "quoted":
					ext = "quoted"
				case "asciihex":
					ext = "asciihex"
				default: // "binary"
					ext = "bin"
				}
				
				sectorDataPath := filepath.Join(trackDir, fmt.Sprintf("sector-%d.%s", sectorNum, ext))
				if err := writer(sectorDataPath, sector.Data); err != nil {
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

