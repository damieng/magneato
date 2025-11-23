// Magneato by damieng - https://github.com/damieng/magneato
// info.go - Info/dump command implementation
// Dual-licensed under MIT and Apache 2.0

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

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

