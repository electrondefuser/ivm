package armv8

import (
	"ivm/src/vm"
	"testing"
)

func TestDecodeAArch64Instructions(t *testing.T) {
	tests := []struct {
		name     string
		raw      uint32
		pc       uint32
		wantOp   vm.Opcode
		wantRd   uint8
		wantImm  uint64
		want64   bool
		wantCond uint8
	}{
		{
			name:    "MOVZ X0, #15",
			raw:     0xD28001E0,
			pc:      0x1000,
			wantOp:  vm.OpMOV,
			wantRd:  0,
			wantImm: 15,
			want64:  true,
		},
		{
			name:    "MOVZ X1, #27",
			raw:     0xD2800361,
			pc:      0x1004,
			wantOp:  vm.OpMOV,
			wantRd:  1,
			wantImm: 27,
			want64:  true,
		},
		{
			name:   "ADD X0, X0, X1",
			raw:    0x8B010000,
			pc:     0x1008,
			wantOp: vm.OpADD,
			wantRd: 0,
			want64: true,
		},
		{
			name:    "MOVZ X8, #93 (sys_exit)",
			raw:     0xD2800BA8,
			pc:      0x100C,
			wantOp:  vm.OpMOV,
			wantRd:  8,
			wantImm: 93,
			want64:  true,
		},
		{
			name:    "SVC #0",
			raw:     0xD4000001,
			pc:      0x1010,
			wantOp:  vm.OpSVC,
			wantImm: 0,
		},
		{
			name:   "NOP",
			raw:    0xD503201F,
			pc:     0x1014,
			wantOp: vm.OpNOP,
		},
		{
			name:   "RET",
			raw:    0xD65F03C0,
			pc:     0x1018,
			wantOp: vm.OpBX,
		},
		{
			name:    "B label (+16 bytes)",
			raw:     0x14000004,
			pc:      0x1000,
			wantOp:  vm.OpB,
			wantImm: 0x1010,
		},
		{
			name:     "B.GT label (+8 bytes)",
			raw:      0x5400004C,
			pc:       0x1000,
			wantOp:   vm.OpB,
			wantCond: vm.CondGT,
			wantImm:  0x1008,
		},
		{
			name:    "STP X29, X30, [SP, #-16]!",
			raw:     0xA9BF7BFD,
			pc:      0x1000,
			wantOp:  vm.OpSTP,
			wantRd:  29,
			want64:  true,
			wantImm: 0xFFFFFFFFFFFFFFF0,
		},
		{
			name:    "LDP X29, X30, [SP], #16",
			raw:     0xA8C17BFD,
			pc:      0x1004,
			wantOp:  vm.OpLDP,
			wantRd:  29,
			want64:  true,
			wantImm: 16,
		},
		{
			name:    "LDR X1, =literal (PC + 28 bytes)",
			raw:     0x580000E1,
			pc:      0x1000,
			wantOp:  vm.OpLDRL,
			wantRd:  1,
			want64:  true,
			wantImm: 0x101C,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := DecodeAArch64(tt.raw, tt.pc)
			if err != nil {
				t.Fatalf("DecodeAArch64 returned unexpected error: %v", err)
			}
			if dec.Op != tt.wantOp {
				t.Errorf("Op mismatch: got %v, want %v", dec.Op, tt.wantOp)
			}
			if tt.wantRd != 0 && dec.Rd != tt.wantRd {
				t.Errorf("Rd mismatch: got %d, want %d", dec.Rd, tt.wantRd)
			}
			if tt.wantImm != 0 && dec.Imm != tt.wantImm {
				t.Errorf("Imm mismatch: got 0x%X, want 0x%X", dec.Imm, tt.wantImm)
			}
			if tt.want64 && !dec.Is64Bit {
				t.Errorf("Is64Bit mismatch: expected true")
			}
			if tt.wantCond != 0 && dec.Cond != tt.wantCond {
				t.Errorf("Cond mismatch: got %d, want %d", dec.Cond, tt.wantCond)
			}
		})
	}
}

func TestUnvirtualizedARM8FallbackEscape(t *testing.T) {
	rawSDIV := uint32(0x9AC10C00)
	dec, err := DecodeAArch64(rawSDIV, 0x1000)
	if err != nil {
		t.Fatalf("DecodeAArch64 failed: %v", err)
	}
	if dec.Op != vm.OpESCAPE {
		t.Errorf("Expected OpESCAPE for unvirtualized SDIV, got %v", dec.Op)
	}
	if uint32(dec.Imm) != rawSDIV {
		t.Errorf("Expected Imm to preserve raw instruction, got 0x%08X", dec.Imm)
	}
}
