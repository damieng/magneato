// Magneato by damieng - https://github.com/damieng/magneato
// types.go - Type definitions for DSK disk image structures
// Dual-licensed under MIT and Apache 2.0

package main

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

// DSK represents the parsed disk image
type DSK struct {
	Header DiskHeader
	Tracks []LogicalTrack // Flat list, can be mapped to Cyl/Head via Header info
}
