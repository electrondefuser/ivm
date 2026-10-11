package armv8

import (
	"ivm/src/vm"
)

// DecodedARM8 represents a decoded 64-bit ARMv8 (AArch64) instruction
type DecodedARM8 struct {
	Op       vm.Opcode
	Cond     uint8  // 0-14
	SetFlags bool   // Updates N, Z, C, V
	HasImm   bool   // True if immediate operand used
	Is64Bit  bool   // True if 64-bit (X register), false if 32-bit (W register)
	Rd       uint8  // 0-31
	Rn       uint8  // 0-31
	Rm       uint8  // 0-31
	Rt2      uint8  // Second register for STP / LDP
	Imm      uint64 // Immediate value or target PC address
	Raw      uint32 // Original 32-bit machine instruction
}

func signExtend26(val uint32) int64 {
	if (val & 0x02000000) != 0 {
		return int64(int32(val | 0xFC000000))
	}
	return int64(val & 0x03FFFFFF)
}

func signExtend19(val uint32) int64 {
	if (val & 0x00040000) != 0 {
		return int64(int32(val | 0xFFF80000))
	}
	return int64(val & 0x0007FFFF)
}

func signExtend07(val uint32) int64 {
	if (val & 0x40) != 0 {
		return int64(int32(val | 0xFFFFFF80))
	}
	return int64(val & 0x7F)
}

// DecodeAArch64 decodes a 32-bit ARMv8 instruction into DecodedARM8
func DecodeAArch64(raw uint32, currPC uint32) (*DecodedARM8, error) {
	dec := &DecodedARM8{
		Raw:  raw,
		Cond: vm.CondAL,
	}

	// 1. SVC #imm16 (Supervisor Call / Syscall)
	if (raw & 0xFFE0001F) == 0xD4000001 {
		dec.Op = vm.OpSVC
		dec.HasImm = true
		dec.Imm = uint64((raw >> 5) & 0xFFFF)
		return dec, nil
	}

	// 2. NOP (D503201F)
	if raw == 0xD503201F {
		dec.Op = vm.OpNOP
		return dec, nil
	}

	// 3. RET (or RET Xn) / BR Xn
	if (raw & 0xFFFFFC1F) == 0xD65F0000 { // RET
		dec.Op = vm.OpBX
		dec.Rn = uint8((raw >> 5) & 0x1F)
		if dec.Rn == 0 {
			dec.Rn = 30 // Default LR is X30
		}
		return dec, nil
	}
	if (raw & 0xFFFFFC1F) == 0xD61F0000 { // BR
		dec.Op = vm.OpBX
		dec.Rn = uint8((raw >> 5) & 0x1F)
		return dec, nil
	}
	if (raw & 0xFFFFFC1F) == 0xD63F0000 { // BLR
		dec.Op = vm.OpBLX
		dec.Rn = uint8((raw >> 5) & 0x1F)
		return dec, nil
	}

	// 4. Branch Immediate: B label (14xxxxxx) / BL label (94xxxxxx)
	if (raw & 0xFC000000) == 0x14000000 { // B
		dec.Op = vm.OpB
		dec.HasImm = true
		imm26 := raw & 0x03FFFFFF
		relOffset := signExtend26(imm26) << 2
		dec.Imm = uint64(int64(currPC) + relOffset)
		return dec, nil
	}
	if (raw & 0xFC000000) == 0x94000000 { // BL
		dec.Op = vm.OpBL
		dec.HasImm = true
		imm26 := raw & 0x03FFFFFF
		relOffset := signExtend26(imm26) << 2
		dec.Imm = uint64(int64(currPC) + relOffset)
		return dec, nil
	}

	// 5. Conditional Branch: B.cond label (54xxxxxx)
	if (raw & 0xFF000010) == 0x54000000 {
		dec.Op = vm.OpB
		dec.HasImm = true
		dec.Cond = uint8(raw & 0x0F)
		imm19 := (raw >> 5) & 0x0007FFFF
		relOffset := signExtend19(imm19) << 2
		dec.Imm = uint64(int64(currPC) + relOffset)
		return dec, nil
	}

	// 6. MOVZ / MOVN (Move wide immediate)
	if (raw&0x7F800000) == 0x52800000 || (raw&0x7F800000) == 0x12800000 {
		isMOVN := (raw & 0x7F800000) == 0x12800000
		dec.Op = vm.OpMOV
		dec.HasImm = true
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.Rd = uint8(raw & 0x1F)
		hw := (raw >> 21) & 0x03
		imm16 := uint64((raw >> 5) & 0xFFFF)
		shift := hw * 16
		val := imm16 << shift
		if isMOVN {
			if dec.Is64Bit {
				val = ^val
			} else {
				val = uint64(^uint32(val))
			}
		}
		dec.Imm = val
		return dec, nil
	}

	// 7. MOV (Register) / ORR Xd, XZR, Xm
	if (raw&0x7FE0FFE0) == 0xAA0003E0 || (raw&0x7FE0FFE0) == 0x2A0003E0 {
		dec.Op = vm.OpMOV
		dec.HasImm = false
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.Rd = uint8(raw & 0x1F)
		dec.Rm = uint8((raw >> 16) & 0x1F)
		return dec, nil
	}

	// 8. ADD / SUB / ADDS / SUBS (Immediate)
	if (raw & 0x1F000000) == 0x11000000 {
		dec.HasImm = true
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.SetFlags = (raw & (1 << 29)) != 0
		isSub := (raw & (1 << 30)) != 0

		dec.Rd = uint8(raw & 0x1F)
		dec.Rn = uint8((raw >> 5) & 0x1F)

		imm12 := uint64((raw >> 10) & 0x0FFF)
		shift := (raw & (1 << 22)) != 0
		if shift {
			imm12 <<= 12
		}
		dec.Imm = imm12

		if isSub {
			if dec.SetFlags && dec.Rd == 31 { // CMP Rn, #imm
				dec.Op = vm.OpCMP
			} else {
				dec.Op = vm.OpSUB
			}
		} else {
			if dec.SetFlags && dec.Rd == 31 { // CMN Rn, #imm
				dec.Op = vm.OpCMN
			} else {
				dec.Op = vm.OpADD
			}
		}
		return dec, nil
	}

	// 9. ADD / SUB / ADDS / SUBS (Shifted Register)
	if (raw & 0x1F200000) == 0x0B000000 {
		dec.HasImm = false
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.SetFlags = (raw & (1 << 29)) != 0
		isSub := (raw & (1 << 30)) != 0

		dec.Rd = uint8(raw & 0x1F)
		dec.Rn = uint8((raw >> 5) & 0x1F)
		dec.Rm = uint8((raw >> 16) & 0x1F)

		if isSub {
			if dec.SetFlags && dec.Rd == 31 { // CMP Rn, Rm
				dec.Op = vm.OpCMP
			} else {
				dec.Op = vm.OpSUB
			}
		} else {
			if dec.SetFlags && dec.Rd == 31 { // CMN Rn, Rm
				dec.Op = vm.OpCMN
			} else {
				dec.Op = vm.OpADD
			}
		}
		return dec, nil
	}

	// 10. MUL (Multiply)
	if (raw & 0x7FE08000) == 0x1B000000 {
		dec.Op = vm.OpMUL
		dec.HasImm = false
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.Rd = uint8(raw & 0x1F)
		dec.Rn = uint8((raw >> 5) & 0x1F)
		dec.Rm = uint8((raw >> 16) & 0x1F)
		return dec, nil
	}

	// 11. Logical: AND / ORR / EOR / BIC (Register)
	if (raw & 0x1F200000) == 0x0A000000 {
		dec.HasImm = false
		dec.Is64Bit = (raw & (1 << 31)) != 0
		opc := (raw >> 29) & 0x03
		dec.Rd = uint8(raw & 0x1F)
		dec.Rn = uint8((raw >> 5) & 0x1F)
		dec.Rm = uint8((raw >> 16) & 0x1F)

		switch opc {
		case 0:
			dec.Op = vm.OpAND
		case 1:
			dec.Op = vm.OpORR
		case 2:
			dec.Op = vm.OpEOR
		case 3:
			dec.Op = vm.OpAND
			dec.SetFlags = true // TST
		}
		return dec, nil
	}

	// 12. LDR (Literal / PC-relative): LDR Xt, label (58xxxxxx) or LDR Wt, label (18xxxxxx)
	if (raw & 0x3F000000) == 0x18000000 {
		dec.Op = vm.OpLDRL
		dec.Is64Bit = (raw & (1 << 30)) != 0
		dec.Rd = uint8(raw & 0x1F)
		dec.HasImm = true
		imm19 := (raw >> 5) & 0x7FFFF
		relOffset := signExtend19(imm19) << 2
		dec.Imm = uint64(int64(currPC) + relOffset)
		return dec, nil
	}

	// 13. Load / Store Single Register (Unsigned Offset)
	// Mask: 0xFFC00000
	if (raw & 0x3B000000) == 0x39000000 {
		dec.HasImm = true
		size := (raw >> 30) & 0x03
		isLoad := (raw & (1 << 22)) != 0
		dec.Rd = uint8(raw & 0x1F)
		dec.Rn = uint8((raw >> 5) & 0x1F)
		imm12 := (raw >> 10) & 0x0FFF

		offset := uint64(imm12) << size
		dec.Imm = offset

		if isLoad {
			switch size {
			case 0:
				dec.Op = vm.OpLDRB
			case 1:
				dec.Op = vm.OpLDRH
			case 2, 3:
				dec.Op = vm.OpLDR
				dec.Is64Bit = (size == 3)
			}
		} else {
			switch size {
			case 0:
				dec.Op = vm.OpSTRB
			case 1:
				dec.Op = vm.OpSTRH
			case 2, 3:
				dec.Op = vm.OpSTR
				dec.Is64Bit = (size == 3)
			}
		}
		return dec, nil
	}

	// 14. STP / LDP (Store / Load Pair)
	// STP X29, X30, [SP, #-16]! / LDP X29, X30, [SP], #16
	if (raw & 0x3E000000) == 0x28000000 {
		isLoad := (raw & (1 << 22)) != 0
		dec.Is64Bit = (raw & (1 << 31)) != 0
		dec.Rd = uint8(raw & 0x1F)
		dec.Rt2 = uint8((raw >> 10) & 0x1F)
		dec.Rm = dec.Rt2 // store second register in Rm
		dec.Rn = uint8((raw >> 5) & 0x1F)
		imm7 := (raw >> 15) & 0x7F
		scale := uint32(2)
		if dec.Is64Bit {
			scale = 3
		}
		dec.Imm = uint64(signExtend07(imm7) << scale)
		dec.HasImm = true

		if isLoad {
			dec.Op = vm.OpLDP
		} else {
			dec.Op = vm.OpSTP
		}
		return dec, nil
	}

	// 14. Fallback for unvirtualized ARMv8 instructions
	dec.Op = vm.OpESCAPE
	dec.HasImm = true
	dec.Imm = uint64(raw)
	return dec, nil
}
