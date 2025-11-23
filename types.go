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

// ==========================================
// Constants and Spec Definitions
// ==========================================

const (
	HeaderSize        = 0x100 // 256 bytes
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

