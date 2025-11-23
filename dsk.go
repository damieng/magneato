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
	"encoding/hex"
	"encoding/json"
	"fmt"
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

