package armv8

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
)

type ELFBinary struct {
	Entry        uint32
	CodeAddr     uint32
	CodeBytes    []byte
	DataAddr     uint32
	DataBytes    []byte
	BSSSize      uint32
	IsExecutable bool
}

// ParseELF reads an ARMv8 (AArch64) ELF binary or object file
func ParseELF(filepath string) (*ELFBinary, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	defer f.Close()

	elfFile, err := elf.NewFile(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ELF header: %w", err)
	}
	defer elfFile.Close()

	if elfFile.Machine != elf.EM_AARCH64 {
		return nil, fmt.Errorf("ELF machine is not AArch64 (EM_AARCH64 = %d), got %v", elf.EM_AARCH64, elfFile.Machine)
	}

	res := &ELFBinary{
		Entry:        uint32(elfFile.Entry),
		IsExecutable: elfFile.Type == elf.ET_EXEC || elfFile.Type == elf.ET_DYN,
	}

	var codeSection *elf.Section
	var dataSections []*elf.Section
	var bssSection *elf.Section

	for _, sec := range elfFile.Sections {
		if sec.Type == elf.SHT_PROGBITS && (sec.Flags&elf.SHF_EXECINSTR) != 0 {
			if codeSection == nil || sec.Name == ".text" {
				codeSection = sec
			}
		} else if sec.Type == elf.SHT_PROGBITS && (sec.Flags&elf.SHF_ALLOC) != 0 {
			dataSections = append(dataSections, sec)
		} else if sec.Type == elf.SHT_NOBITS && (sec.Flags&elf.SHF_ALLOC) != 0 {
			bssSection = sec
		}
	}

	if codeSection == nil {
		return nil, fmt.Errorf("no executable section (.text) found in AArch64 ELF")
	}

	codeData, err := codeSection.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read code section data: %w", err)
	}

	res.CodeBytes = codeData
	if codeSection.Addr != 0 {
		res.CodeAddr = uint32(codeSection.Addr)
	} else {
		res.CodeAddr = 0x00010000 // Default base address for relocatable object files
	}

	if res.Entry == 0 {
		symbols, _ := elfFile.Symbols()
		for _, sym := range symbols {
			if sym.Name == "_start" || (res.Entry == 0 && sym.Name == "main") {
				res.Entry = res.CodeAddr + uint32(sym.Value)
				if sym.Name == "_start" {
					break
				}
			}
		}
		if res.Entry == 0 {
			res.Entry = res.CodeAddr
		}
	}

	// Consolidate data sections
	if len(dataSections) > 0 {
		firstSec := dataSections[0]
		res.DataAddr = uint32(firstSec.Addr)
		if res.DataAddr == 0 {
			res.DataAddr = 0x00020000
		}

		var combinedData []byte
		for _, ds := range dataSections {
			d, err := ds.Data()
			if err != nil {
				return nil, fmt.Errorf("failed to read data section %s: %w", ds.Name, err)
			}
			combinedData = append(combinedData, d...)
		}
		res.DataBytes = combinedData
	}

	if bssSection != nil {
		res.BSSSize = uint32(bssSection.Size)
	}

	// Apply relocations for ET_REL AArch64 object files
	if elfFile.Type == elf.ET_REL {
		symbols, _ := elfFile.Symbols()
		applyRelocations(elfFile, codeSection, res, symbols)
	}

	return res, nil
}

func applyRelocations(elfFile *elf.File, codeSec *elf.Section, bin *ELFBinary, _ []elf.Symbol) {
	var codeSecIdx int
	for idx, sec := range elfFile.Sections {
		if sec == codeSec {
			codeSecIdx = idx
			break
		}
	}

	var symtabData []byte
	for _, sec := range elfFile.Sections {
		if sec.Type == elf.SHT_SYMTAB {
			symtabData, _ = sec.Data()
			break
		}
	}

	// Helper for 64-bit ELF symbol lookup (each entry is 24 bytes in ELF64)
	getSymAddr := func(symIdx uint32) uint64 {
		if len(symtabData) < int((symIdx+1)*24) {
			return 0
		}
		offset := symIdx * 24
		stValue := binary.LittleEndian.Uint64(symtabData[offset+8 : offset+16])
		stShndx := binary.LittleEndian.Uint16(symtabData[offset+6 : offset+8])

		if int(stShndx) < len(elfFile.Sections) {
			secName := elfFile.Sections[stShndx].Name
			if secName == ".text" {
				return uint64(bin.CodeAddr) + stValue
			} else if secName == ".data" || secName == ".rodata" {
				return uint64(bin.DataAddr) + stValue
			}
		}
		return uint64(bin.CodeAddr) + stValue
	}

	for _, sec := range elfFile.Sections {
		// AArch64 primarily uses SHT_RELA (with explicit addend, 24 bytes per entry)
		if sec.Type == elf.SHT_RELA && sec.Info == uint32(codeSecIdx) {
			relaData, err := sec.Data()
			if err != nil {
				continue
			}

			for i := 0; i+24 <= len(relaData); i += 24 {
				rOffset := binary.LittleEndian.Uint64(relaData[i : i+8])
				rInfo := binary.LittleEndian.Uint64(relaData[i+8 : i+16])
				rAddend := int64(binary.LittleEndian.Uint64(relaData[i+16 : i+24]))

				symIdx := uint32(rInfo >> 32)
				rType := uint32(rInfo & 0xFFFFFFFF)

				symAddr := getSymAddr(symIdx)

				// R_AARCH64_CALL26 = 283, R_AARCH64_JUMP26 = 282 (B and BL instructions)
				if rType == 283 || rType == 282 {
					if rOffset+4 <= uint64(len(bin.CodeBytes)) {
						rawInst := binary.LittleEndian.Uint32(bin.CodeBytes[rOffset : rOffset+4])
						p := uint64(bin.CodeAddr) + rOffset
						s := symAddr

						relBytes := int64(s) + rAddend - int64(p)
						imm26 := uint32((relBytes >> 2) & 0x03FFFFFF)

						rawInst = (rawInst & 0xFC000000) | imm26
						binary.LittleEndian.PutUint32(bin.CodeBytes[rOffset:rOffset+4], rawInst)
					}
				}

				// R_AARCH64_ABS64 = 257 (64-bit absolute pointer)
				if rType == 257 {
					if rOffset+8 <= uint64(len(bin.CodeBytes)) {
						finalVal := uint64(int64(symAddr) + rAddend)
						binary.LittleEndian.PutUint64(bin.CodeBytes[rOffset:rOffset+8], finalVal)
					}
				}

				// R_AARCH64_ABS32 = 258 (32-bit absolute pointer)
				if rType == 258 {
					if rOffset+4 <= uint64(len(bin.CodeBytes)) {
						finalVal := uint32(int64(symAddr) + rAddend)
						binary.LittleEndian.PutUint32(bin.CodeBytes[rOffset:rOffset+4], finalVal)
					}
				}
			}
		}
	}
}
