package vm

import (
	"ivm/src/format"
	"ivm/src/isa"
	"testing"
)

func TestVMExecutionAdd(t *testing.T) {
	// Program: MOV R0, #15; MOV R1, #27; ADD R0, R0, R1; MOV R7, #1; SVC #0
	instructions := []isa.Instruction{
		{Op: isa.OpMOV, Cond: isa.CondAL, RegDst: isa.R0, HasImm: true, Imm: 15},
		{Op: isa.OpMOV, Cond: isa.CondAL, RegDst: isa.R1, HasImm: true, Imm: 27},
		{Op: isa.OpADD, Cond: isa.CondAL, RegDst: isa.R0, RegSrc1: isa.R0, RegSrc2: isa.R1},
		{Op: isa.OpMOV, Cond: isa.CondAL, RegDst: isa.R7, HasImm: true, Imm: 1},
		{Op: isa.OpSVC, Cond: isa.CondAL, Imm: 0},
	}

	bytecode := isa.EncodeSlice(instructions)

	ivmFile := &format.IVMFile{
		Header: format.Header{
			Arch:         format.ArchARMv7,
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

	if interp.CPU.GetReg(isa.R0) != 42 {
		t.Errorf("Expected R0 = 42, got %d", interp.CPU.GetReg(isa.R0))
	}
}
