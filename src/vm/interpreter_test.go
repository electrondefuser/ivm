package vm

import (
	"testing"
)

func TestVMExecutionAdd(t *testing.T) {
	// Program: MOV R0, #15; MOV R1, #27; ADD R0, R0, R1; MOV R7, #1; SVC #0
	instructions := []Instruction{
		{Op: OpMOV, Cond: CondAL, RegDst: R0, HasImm: true, Imm: 15},
		{Op: OpMOV, Cond: CondAL, RegDst: R1, HasImm: true, Imm: 27},
		{Op: OpADD, Cond: CondAL, RegDst: R0, RegSrc1: R0, RegSrc2: R1},
		{Op: OpMOV, Cond: CondAL, RegDst: R7, HasImm: true, Imm: 1},
		{Op: OpSVC, Cond: CondAL, Imm: 0},
	}

	bytecode := EncodeSlice(instructions)

	ivmFile := &IVMFile{
		Header: Header{
			Arch:         ArchARMv7,
			CodeLoadAddr: 0x00010000,
			EntryPC:      0x00010000,
			CodeSize:     uint32(len(bytecode)),
		},
		Bytecode: bytecode,
	}

	interp := NewInterpreter(false)
	err := interp.LoadIVMFile(ivmFile)
	if err != nil {
		t.Fatalf("Failed to load IVM file: %v", err)
	}

	exitCode, err := interp.Exec()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if exitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", exitCode)
	}

	if interp.CPU.GetReg(R0) != 42 {
		t.Errorf("Expected R0 = 42, got %d", interp.CPU.GetReg(R0))
	}
}
