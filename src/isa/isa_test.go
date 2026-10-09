package isa

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
