// Magneato by damieng - https://github.com/damieng/magneato
// types.go - Type definitions for DSK disk image structures
// Dual-licensed under MIT and Apache 2.0

package main

// Constants
const (
	HeaderSize        = 0x100 // 256 bytes
	TrackSizeTableLen = 204   // 256 - 0x34 - 0x1E approx, but spec defines fixed offset
)

// DiskHeader represents the 256-byte file header
// Layout:
//   0x00-0x21: SignatureString (34 bytes)
//   0x22-0x2F: CreatorString (14 bytes)
//   0x30: Tracks (1 byte)
//   0x31: Sides (1 byte)
//   0x32-0x33: Padding (2 bytes)
//   0x34-0xFF: TrackSizeTable (204 bytes)
type DiskHeader struct {
	SignatureString [34]byte // "EXTENDED CPC DSK File\r\nDisk-Info\r\n"
	CreatorString   [14]byte
	Tracks          uint8 // Number of tracks (cylinders)
	Sides           uint8 // Number of sides
	_               [2]byte // Padding to align TrackSizeTable at offset 0x34
	TrackSizeTable  [204]uint8 // High byte of track sizes (starts at offset 0x34)
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
// Layout per spec:
//   00-0c: "Track-Info\r\n" (13 bytes)
//   0d-0f: unused (3 bytes)
//   10: track number (1 byte)
//   11: side number (1 byte)
//   12-13: unused (2 bytes)
//   14: sector size (1 byte)
//   15: number of sectors (1 byte)
//   16: GAP#3 length (1 byte)
//   17: filler byte (1 byte)
type TrackHeader struct {
	Signature    [13]byte // "Track-Info\r\n" (13 bytes, not 12!)
	Unused       [3]byte  // unused (3 bytes, not 4!)
	TrackNum     uint8
	SideNum      uint8
	Unused2      [2]byte
	SectorSize   uint8 // Defined sector size (N)
	SectorCount  uint8
	Gap3Length   uint8
	FillerByte   uint8
}

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

// DSKFormat represents the type of DSK format
type DSKFormat int

const (
	// FormatExtended represents the Extended CPC DSK format
	FormatExtended DSKFormat = iota
	// FormatStandard represents the Standard DSK format
	FormatStandard
)

// SpecificationFormat represents the disk format type in the specification block
type SpecificationFormat int

const (
	// SpecFormatPCW_SS represents Amstrad PCW single-sided format
	SpecFormatPCW_SS SpecificationFormat = iota
	// SpecFormatCPC_System represents Amstrad CPC system format
	SpecFormatCPC_System
	// SpecFormatCPC_Data represents Amstrad CPC data format
	SpecFormatCPC_Data
	// SpecFormatPCW_DS represents Amstrad PCW double-sided format
	SpecFormatPCW_DS
)

// SpecificationSide represents the side configuration in the specification block
type SpecificationSide int

const (
	// SpecSideSingle represents single-sided disk
	SpecSideSingle SpecificationSide = iota
	// SpecSideDoubleAlternate represents double-sided alternate format
	SpecSideDoubleAlternate
	// SpecSideDoubleSuccessive represents double-sided successive format
	SpecSideDoubleSuccessive
)

// SpecificationTrack represents the track density in the specification block
type SpecificationTrack int

const (
	// SpecTrackSingle represents single density tracks
	SpecTrackSingle SpecificationTrack = iota
	// SpecTrackDouble represents double density tracks
	SpecTrackDouble
)

// Specification represents the disk format specification block
// This is typically stored in the first 16 bytes of sector 0, track 0, side 0
// Layout per TDSKSpecification from DiskImageManager:
//   0: Format (0=PCW_SS, 1=CPC_System, 2=CPC_Data, 3=PCW_DS)
//   1: Side configuration (bits 0-1: 0=Single, 1=DoubleAlternate, 2=DoubleSuccessive; bit 7: Track density)
//   2: TracksPerSide
//   3: SectorsPerTrack
//   4: SectorSize (log2(sectorSize) - 7, so 0=128, 1=256, 2=512, etc.)
//   5: ReservedTracks
//   6: BlockShift
//   7: DirectoryBlocks
//   8: GapReadWrite
//   9: GapFormat
//   10-14: Reserved (0)
//   15: Checksum
type Specification struct {
	Format         SpecificationFormat // Disk format type
	Side           SpecificationSide    // Side configuration
	Track          SpecificationTrack   // Track density
	TracksPerSide  uint8                // Number of tracks per side
	SectorsPerTrack uint8                // Number of sectors per track
	SectorSize     uint16               // Sector size in bytes (calculated from N)
	ReservedTracks uint8                // Number of reserved tracks
	BlockShift     uint8                // Block shift value
	DirectoryBlocks uint8                // Number of directory blocks
	GapReadWrite   uint8                // GAP#3 length for read/write
	GapFormat      uint8                // GAP#3 length for format
	Checksum       uint8                // Checksum byte
}

// DSK represents the parsed disk image
type DSK struct {
	Format DSKFormat // Standard or Extended format
	Header DiskHeader
	Tracks []LogicalTrack // Flat list, can be mapped to Cyl/Head via Header info
	// For standard format, this stores the fixed track size
	StandardTrackSize uint16
	// Specification block (if present, typically in sector 0, track 0, side 0)
	Specification *Specification
}
