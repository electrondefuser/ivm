package vm

import (
	"testing"
)

func TestInstructionEncodingDecoding(t *testing.T) {
	original := Instruction{
		Op:          OpADD,
		Cond:        CondEQ,
		SetFlags:    true,
		HasImm:      true,
		ShiftType:   ShiftLSL,
		ShiftAmount: 4,
		RegDst:      R0,
		RegSrc1:     R1,
		RegSrc2:     R2,
		Imm:         0xDEADBEEF,
	}

	encoded := EncodeInstruction(original)
	decoded := DecodeInstruction(encoded)

	if decoded.Op != original.Op {
		t.Errorf("Opcode mismatch: expected %v, got %v", original.Op, decoded.Op)
	}
	if decoded.Cond != original.Cond {
		t.Errorf("Cond mismatch: expected %v, got %v", original.Cond, decoded.Cond)
	}
	if decoded.SetFlags != original.SetFlags {
		t.Errorf("SetFlags mismatch: expected %v, got %v", original.SetFlags, decoded.SetFlags)
	}
	if decoded.HasImm != original.HasImm {
		t.Errorf("HasImm mismatch: expected %v, got %v", original.HasImm, decoded.HasImm)
	}
	if decoded.RegDst != original.RegDst {
		t.Errorf("RegDst mismatch: expected %v, got %v", original.RegDst, decoded.RegDst)
	}
	if decoded.RegSrc1 != original.RegSrc1 {
		t.Errorf("RegSrc1 mismatch: expected %v, got %v", original.RegSrc1, decoded.RegSrc1)
	}
	if decoded.Imm != original.Imm {
		t.Errorf("Imm mismatch: expected 0x%X, got 0x%X", original.Imm, decoded.Imm)
	}
}

func TestARMv8InstructionEncodingDecoding(t *testing.T) {
	original := Instruction{
		Op:       OpADD,
		Cond:     CondAL,
		SetFlags: false,
		HasImm:   false,
		Is64Bit:  true,
		Arch:     ArchARMv8,
		RegDst:   29, // FP (X29)
		RegSrc1:  30, // LR (X30)
		RegSrc2:  31, // SP (X31)
		Imm:      128,
	}

	encoded := EncodeInstruction(original)
	decoded := DecodeInstruction(encoded, ArchARMv8)

	if decoded.Op != original.Op {
		t.Errorf("Opcode mismatch: expected %v, got %v", original.Op, decoded.Op)
	}
	if decoded.Is64Bit != original.Is64Bit {
		t.Errorf("Is64Bit mismatch: expected %v, got %v", original.Is64Bit, decoded.Is64Bit)
	}
	if decoded.RegDst != 29 {
		t.Errorf("RegDst mismatch: expected 29, got %v", decoded.RegDst)
	}
	if decoded.RegSrc1 != 30 {
		t.Errorf("RegSrc1 mismatch: expected 30, got %v", decoded.RegSrc1)
	}
	if decoded.RegSrc2 != 31 {
		t.Errorf("RegSrc2 mismatch: expected 31, got %v", decoded.RegSrc2)
	}
	if decoded.Imm != 128 {
		t.Errorf("Imm mismatch: expected 128, got %v", decoded.Imm)
	}
}
