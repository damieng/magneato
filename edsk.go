package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ==========================================
// Constants and Spec Definitions
// ==========================================

const (
	HeaderSize      = 0x100 // 256 bytes
	TrackSizeTableLen = 204   // 256 - 0x34 - 0x1E approx, but spec defines fixed offset
)

// DiskHeader represents the 256-byte file header
type DiskHeader struct {
	SignatureString [34]byte // "EXTENDED CPC DSK File\r\nDisk-Info\r\n"
	CreatorString   [14]byte
	Tracks          uint8 // Number of tracks (cylinders)
	Sides           uint8 // Number of sides
	TrackSizeTable  [204]uint8 // High byte of track sizes
}

// SectorInfo represents the 8-byte sector descriptor in the Track Block
type SectorInfo struct {
	C          uint8  // Cylinder
	H          uint8  // Head
	R          uint8  // Sector ID
	N          uint8  // Sector Size (128 * 2^N)
	FDCStatus1 uint8  // FDC Status Register 1
	FDCStatus2 uint8  // FDC Status Register 2
	DataLength uint16 // actual data length in bytes (Little Endian)
}

// TrackHeader represents the start of a track block
type TrackHeader struct {
	Signature    [12]byte // "Track-Info\r\n"
	Unused       [4]byte
	TrackNum     uint8
	SideNum      uint8
	Unused2      [2]byte
	SectorSize   uint8 // Defined sector size (N)
	SectorCount  uint8
	Gap3Length   uint8
	FillerByte   uint8
}

// ==========================================
// Logical Abstractions
// ==========================================

// LogicalSector contains the metadata and the actual payload
type LogicalSector struct {
	Info SectorInfo
	Data []byte
}

// LogicalTrack contains the track metadata and a slice of sectors
type LogicalTrack struct {
	Header  TrackHeader
	Sectors []LogicalSector
}

// DSK represents the parsed disk image
type DSK struct {
	Header DiskHeader
	Tracks []LogicalTrack // Flat list, can be mapped to Cyl/Head via Header info
}

// ==========================================
// Parser Implementation
// ==========================================

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

		// Check Track Signature
		if string(tHeader.Signature[:10]) != "Track-Info" {
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

// ==========================================
// Helper Methods for Abstraction
// ==========================================

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
func (d *DSK) Unpack(dskFilename string) error {
	// Get base name without extension
	baseName := strings.TrimSuffix(filepath.Base(dskFilename), filepath.Ext(dskFilename))
	
	// Create root directory
	rootDir := baseName
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory: %v", err)
	}
	
	// Create disk-image.meta
	diskMeta := map[string]interface{}{
		"signature": string(bytes.Trim(d.Header.SignatureString[:], "\x00")),
		"creator":   string(bytes.Trim(d.Header.CreatorString[:], "\x00")),
		"tracks":    d.Header.Tracks,
		"sides":     d.Header.Sides,
	}
	
	diskMetaPath := filepath.Join(rootDir, "disk-image.meta")
	diskMetaJSON, err := json.MarshalIndent(diskMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal disk metadata: %v", err)
	}
	if err := os.WriteFile(diskMetaPath, diskMetaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write disk metadata: %v", err)
	}
	
	// Process each track
	for i, track := range d.Tracks {
		// Create track directory (format: track-XX-side-Y or track-XX)
		trackDirName := fmt.Sprintf("track-%02d", i)
		if d.Header.Sides > 1 {
			trackDirName = fmt.Sprintf("track-%02d-side-%d", track.Header.TrackNum, track.Header.SideNum)
		}
		trackDir := filepath.Join(rootDir, trackDirName)
		
		if err := os.MkdirAll(trackDir, 0755); err != nil {
			return fmt.Errorf("failed to create track directory: %v", err)
		}
		
		// Create track.meta
		trackMeta := map[string]interface{}{
			"track_number": track.Header.TrackNum,
			"side_number":  track.Header.SideNum,
			"sector_size":  track.Header.SectorSize,
			"sector_count": track.Header.SectorCount,
			"gap3_length":  track.Header.Gap3Length,
			"filler_byte":  track.Header.FillerByte,
		}
		
		trackMetaPath := filepath.Join(trackDir, "track.meta")
		trackMetaJSON, err := json.MarshalIndent(trackMeta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal track metadata: %v", err)
		}
		if err := os.WriteFile(trackMetaPath, trackMetaJSON, 0644); err != nil {
			return fmt.Errorf("failed to write track metadata: %v", err)
		}
		
		// Process each sector in the track
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
	
	fmt.Printf("Successfully unpacked DSK to: %s\n", rootDir)
	return nil
}

// ==========================================
// Main Entry Point
// ==========================================

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run edsk.go <command> <filename.dsk>")
		fmt.Println("Commands:")
		fmt.Println("  info    - Display DSK file information")
		fmt.Println("  unpack  - Extract DSK to directory structure")
		os.Exit(1)
	}

	command := os.Args[1]
	filename := os.Args[2]
	
	fmt.Printf("Processing file: %s\n", filename)

	dsk, err := ParseDSK(filename)
	if err != nil {
		log.Fatalf("Error parsing DSK: %v", err)
	}

	switch command {
	case "info":
		dsk.DumpInfo()
	case "unpack":
		if err := dsk.Unpack(filename); err != nil {
			log.Fatalf("Error unpacking DSK: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Commands: info, unpack")
		os.Exit(1)
	}
}