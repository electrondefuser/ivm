package armv8

import (
	"encoding/binary"
	"fmt"
	"ivm/src/vm"
)

type TranslationResult struct {
	IVMFile *vm.IVMFile
	Stats   TranslationStats
}

type TranslationStats struct {
	ARMInstructionCount int
	IVMInstructionCount int
}

// TranslateELF converts an ARMv8 ELF binary into an IVM File container.
func TranslateELF(elfBin *ELFBinary) (*TranslationResult, error) {
	codeBytes := elfBin.CodeBytes
	if len(codeBytes)%4 != 0 {
		return nil, fmt.Errorf("code section length (%d bytes) is not aligned to 4 bytes", len(codeBytes))
	}

	armInstCount := len(codeBytes) / 4
	ivmInstructions := make([]vm.Instruction, 0, armInstCount)

	baseAddr := elfBin.CodeAddr

	for i := range armInstCount {
		currPC := baseAddr + uint32(i*4)
		rawInst := binary.LittleEndian.Uint32(codeBytes[i*4 : (i+1)*4])

		decARM8, err := DecodeAArch64(rawInst, currPC)
		if err != nil {
			return nil, fmt.Errorf("error at PC 0x%08X (inst #%d, raw 0x%08X): %w", currPC, i, rawInst, err)
		}

		ivmInst := convertDecodedARM8ToIVM(decARM8, currPC)
		ivmInstructions = append(ivmInstructions, ivmInst)
	}

	bytecode := vm.EncodeSlice(ivmInstructions)

	ivmFile := &vm.IVMFile{
		Header: vm.Header{
			Arch:         vm.ArchARMv8,
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

	return &TranslationResult{
		IVMFile: ivmFile,
		Stats: TranslationStats{
			ARMInstructionCount: armInstCount,
			IVMInstructionCount: len(ivmInstructions),
		},
	}, nil
}

func convertDecodedARM8ToIVM(dec *DecodedARM8, currPC uint32) vm.Instruction {
	inst := vm.Instruction{
		Op:       dec.Op,
		Cond:     dec.Cond,
		SetFlags: dec.SetFlags,
		HasImm:   dec.HasImm,
		Is64Bit:  dec.Is64Bit,
		Arch:      vm.ArchARMv8,
		ShiftType: dec.ShiftType,
		RegDst:    dec.Rd,
		RegSrc1:  dec.Rn,
		RegSrc2:  dec.Rm,
		Imm:      uint32(dec.Imm),
	}

	if dec.Cond == 0x0F {
		inst.Cond = vm.CondAL
	}

	return inst
}
