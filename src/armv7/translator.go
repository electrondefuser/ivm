package armv7

import (
	"encoding/binary"
	"fmt"
	"ivm/src/format"
	"ivm/src/isa"
)

type TranslationReslt struct {
	IVMFile *format.IVMFile
	Stats   TranslationStats
}

type TranslationStats struct {
	ARMInstructionCount int
	IVMInstructionCount int
}

// TranslateELF converts an ARMv7 ELF binary into an IVM File container.
func TranslateELF(elfBin *ELFBinary) (*TranslationReslt, error) {
	codeBytes := elfBin.CodeBytes
	if len(codeBytes)%4 != 0 {
		return nil, fmt.Errorf("code section length (%d bytes) is not aligned to 4 bytes", len(codeBytes))
	}

	armInstCount := len(codeBytes) / 4
	ivmInstructions := make([]isa.Instruction, 0, armInstCount)

	baseAddr := elfBin.CodeAddr

	for i := range armInstCount {
		currPC := baseAddr + uint32(i*4)
		rawInst := binary.LittleEndian.Uint32(codeBytes[i*4 : (i+1)*4])

		decARM, err := DecodeARM32(rawInst)
		if err != nil {
			return nil, fmt.Errorf("error at PC 0x%08X (inst #%d, raw 0x%08X): %w", currPC, i, rawInst, err)
		}

		ivmInst := convertDecodedARMToIVM(decARM, currPC)
		ivmInstructions = append(ivmInstructions, ivmInst)
	}

	bytecode := isa.EncodeSlice(ivmInstructions)

	ivmFile := &format.IVMFile{
		Header: format.Header{
			Arch:         format.ArchARMv7,
			EntryPC:      elfBin.Entry,
			CodeLoadAddr: elfBin.CodeAddr,
			CodeSize:     uint32(len(bytecode)),
			RawCodeSize:  uint32(len(codeBytes)),
			DataLoadAddr: elfBin.DataAddr,
			DataSize:     uint32(len(elfBin.DataBytes)),
			BSSSize:      elfBin.BSSSize,
			StackSize:    0x00080000, // 512 KB default stack
		},
		Bytecode: bytecode,
		RawCode:  codeBytes,
		Data:     elfBin.DataBytes,
	}

	return &TranslationReslt{
		IVMFile: ivmFile,
		Stats: TranslationStats{
			ARMInstructionCount: armInstCount,
			IVMInstructionCount: len(ivmInstructions),
		},
	}, nil
}

func convertDecodedARMToIVM(dec *DecodedARM, currPC uint32) isa.Instruction {
	inst := isa.Instruction{
		Op:          dec.Op,
		Cond:        dec.Cond,
		SetFlags:    dec.SetFlags,
		HasImm:      dec.HasImm,
		ShiftType:   dec.ShiftType,
		ShiftAmount: dec.ShiftAmount,
		RegDst:      dec.Rd,
		RegSrc1:     dec.Rn,
		RegSrc2:     dec.Rm,
		Imm:         dec.Imm,
	}

	// Calculate absolute target address for relative branch instructions
	if dec.Op == isa.OpB || dec.Op == isa.OpBL {
		// ARM PC during instruction execution is CurrentPC + 8
		targetAddr := currPC + 8 + uint32(int32(dec.Imm))
		inst.Imm = targetAddr
	}

	// Set default condition to AL if 0 or 14
	if dec.Cond == 0x0F {
		inst.Cond = isa.CondAL
	}

	return inst
}
