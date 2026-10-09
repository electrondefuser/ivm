package armv7

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

/* ParseELF reads an ARMv7 ELF binary or object file */
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

	if elfFile.Machine != elf.EM_ARM {
		return nil, fmt.Errorf("ELF machine is not ARM (expected %d), got %v", elf.EM_ARM, elfFile.Machine)
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
		return nil, fmt.Errorf("no executable section (.text) found in ELF")
	}

	codeData, err := codeSection.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read code section data: %w", err)
	}

	res.CodeBytes = codeData
	if codeSection.Addr != 0 {
		res.CodeAddr = uint32(codeSection.Addr)
	} else {
		res.CodeAddr = 0x00010000 // Default base address for object files
	}

	if res.Entry == 0 {
		res.Entry = res.CodeAddr
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

	// Apply relocations for ET_REL object files
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

	getSymAddr := func(symIdx uint32) uint32 {
		if len(symtabData) < int((symIdx+1)*16) {
			return 0
		}
		offset := symIdx * 16
		stValue := binary.LittleEndian.Uint32(symtabData[offset+4 : offset+8])
		stShndx := binary.LittleEndian.Uint16(symtabData[offset+14 : offset+16])

		if int(stShndx) < len(elfFile.Sections) {
			secName := elfFile.Sections[stShndx].Name
			if secName == ".text" {
				return bin.CodeAddr + stValue
			} else if secName == ".data" || secName == ".rodata" {
				return bin.DataAddr + stValue
			} else {
				return bin.DataAddr + stValue
			}
		}
		return bin.CodeAddr + stValue
	}

	for _, sec := range elfFile.Sections {
		if sec.Type == elf.SHT_REL && sec.Info == uint32(codeSecIdx) {
			relData, err := sec.Data()
			if err != nil {
				continue
			}

			// Each ARM ELF SHT_REL entry is 8 bytes: r_offset (4B), r_info (4B)
			for i := 0; i+8 <= len(relData); i += 8 {
				rOffset := binary.LittleEndian.Uint32(relData[i : i+4])
				rInfo := binary.LittleEndian.Uint32(relData[i+4 : i+8])

				symIdx := rInfo >> 8
				rType := uint8(rInfo & 0xFF)

				symAddr := getSymAddr(symIdx)

				// R_ARM_ABS32 = 2, R_ARM_TARGET1 = 38
				if rType == 2 || rType == 38 {
					if rOffset+4 <= uint32(len(bin.CodeBytes)) {
						addend := binary.LittleEndian.Uint32(bin.CodeBytes[rOffset : rOffset+4])
						finalVal := symAddr + addend
						binary.LittleEndian.PutUint32(bin.CodeBytes[rOffset:rOffset+4], finalVal)
					}
				}

				// R_ARM_CALL = 28, R_ARM_JUMP24 = 29
				if rType == 28 || rType == 29 {
					if rOffset+4 <= uint32(len(bin.CodeBytes)) {
						rawInst := binary.LittleEndian.Uint32(bin.CodeBytes[rOffset : rOffset+4])
						offset24 := rawInst & 0x00FFFFFF
						addend := signExtend24(offset24) << 2

						p := bin.CodeAddr + rOffset
						s := symAddr

						relBytes := int32(s) + addend - int32(p)
						newOffset24 := uint32((relBytes >> 2) & 0x00FFFFFF)

						rawInst = (rawInst & 0xFF000000) | newOffset24
						binary.LittleEndian.PutUint32(bin.CodeBytes[rOffset:rOffset+4], rawInst)
					}
				}
			}
		}
	}
}
