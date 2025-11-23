# Magneato

A command-line tool for parsing and extracting Extended CPC DSK (eDSK) disk image files. Magneato provides a clean interface to inspect, unpack and pack DSK files into a structured directory format.

**Currently experimental**

## Features

- **Parse DSK files**: Read and validate Extended CPC DSK format files
- **Inspect disk images**: Display detailed information about tracks, sectors, and metadata
- **Extract disk images**: Unpack DSK files into a structured directory hierarchy with separate data and metadata files

## Installation

### Prerequisites

- Go 1.16 or later

### Build from Source

```bash
git clone <repository-url>
cd Magneato
go build -o magneato
```

Or run directly:

```bash
go run src/. <command> <filename.dsk>
```

## Usage

Magneato supports three commands:

### Info Command

Display detailed information about a DSK file:

```bash
magneato info disk.dsk
```

This will show:

- Disk signature and creator information
- Number of tracks and sides
- Track-by-track breakdown with sector details
- Sector metadata including FDC status registers

### Unpack Command

Extract a DSK file into a structured directory:

```bash
magneato unpack disk.dsk
```

You can also specify a `--data-format`

This creates a directory structure:

```
disk/
├── disk-image.meta          # Disk-level metadata (JSON)
├── track-00-side-0/
│   ├── track.meta           # Track metadata (JSON)
│   ├── sector-0.bin         # Binary sector data
│   ├── sector-0.meta        # Sector metadata (JSON)
│   ├── sector-1.bin
│   ├── sector-1.meta
│   └── ...
├── track-00-side-1/
│   └── ...
└── ...
```

#### Directory Structure

- **Root directory**: Named after the DSK file (without extension)
- **Track directories**: Named `track-XX-side-Y` for multi-sided disks, or `track-XX` for single-sided
- **Sector files**:
  - `sector-N.meta`: Sector metadata in JSON format
  - `sector-N.bin`: Sector data in raw binary format
  - `sector-N.hex`: Sector data in hex format
  - `sector-N.quote`: Sector data in quoted-printable format
- **Metadata files**:
  - `disk-image.meta`: Disk header information
  - `track.meta`: Track header information

## Pack Command

The reverse of `unpack` this combines the various files back into a .DSK file attempting to preserve precision and minimize data and meta loss.

## File Format

Magneato supports the Extended CPC DSK format, which includes:

- **Disk Header**: 256-byte header with signature, creator info, and track size table
- **Track Blocks**: Variable-length blocks containing track headers and sector data
- **Sector Information**: 8-byte descriptors with cylinder, head, sector ID, and FDC status
- **Sector Data**: Raw sector payloads with variable lengths

## Project Structure

```
.
├── main.go      # Command-line interface and entry point
├── types.go     # Type definitions and constants
├── parser.go    # DSK file parsing logic
├── dsk.go       # DSK methods (info, unpack, etc.)
└── README.md    # This file
```

## License

This project is dual-licensed under:

- **Apache License 2.0** - See [LICENSE-APACHE](LICENSE-APACHE) file for details
- **MIT License** - See [LICENSE-MIT](LICENSE-MIT) file for details

You may choose either license at your option.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## AI Disclosure

This tool with developed using AI tooling from Google (Gemini 3 Pro) and Cursor (default model).
