package armv7

import (
	"fmt"
	"ivm/src/isa"
)

/* DecodedARM represents an ARMv7 instruction disassembled into intermediate representation */
type DecodedARM struct {
	Op          isa.Opcode
	Cond        uint8
	SetFlags    bool
	HasImm      bool
	ShiftType   uint8
	ShiftAmount uint8
	Rd          uint8
	Rn          uint8
	Rm          uint8
	Imm         uint32
	RegList     uint16 // Mask for LDM/STM/PUSH/POP
	Raw         uint32
}

func rotateRight(val uint32, count uint32) uint32 {
	count &= 31
	return (val >> count) | (val << (32 - count))
}

func signExtend24(val uint32) int32 {
	if (val & 0x00800000) != 0 {
		return int32(val | 0xFF000000)
	}
	return int32(val & 0x00FFFFFF)
}

/* DecodeARM32 decodes a 32-bit ARM instruction into a DecodedARM struct */
func DecodeARM32(raw uint32) (*DecodedARM, error) {
	cond := uint8((raw >> 28) & 0x0F)
	dec := &DecodedARM{
		Raw:  raw,
		Cond: cond,
	}

	// 1. SVC / SWI (cond | 1111 | imm24)
	if (raw & 0x0F000000) == 0x0F000000 {
		dec.Op = isa.OpSVC
		dec.HasImm = true
		dec.Imm = raw & 0x00FFFFFF
		return dec, nil
	}

	// 2. Branch & Branch with Link (cond | 101 | L | offset24)
	if (raw & 0x0E000000) == 0x0A000000 {
		link := (raw & 0x01000000) != 0
		if link {
			dec.Op = isa.OpBL
		} else {
			dec.Op = isa.OpB
		}
		offset24 := raw & 0x00FFFFFF
		relOffset := signExtend24(offset24) << 2
		dec.HasImm = true
		dec.Imm = uint32(relOffset)
		return dec, nil
	}

	// 3. BX / BLX Register (cond | 0001 0010 1111 1111 1111 0001/0011 | Rm)
	if (raw & 0x0FFFFFF0) == 0x012FFF10 {
		dec.Op = isa.OpBX
		dec.Rm = uint8(raw & 0x0F)
		dec.Rn = dec.Rm
		return dec, nil
	}

	if (raw & 0x0FFFFFF0) == 0x012FFF30 {
		dec.Op = isa.OpBLX
		dec.Rm = uint8(raw & 0x0F)
		dec.Rn = dec.Rm
		return dec, nil
	}

	// 4. Block Data Transfer (LDM/STM, PUSH/POP) - (cond | 100 | P | U | S | W | L | Rn | RegList)
	if (raw & 0x0E000000) == 0x08000000 {
		lBit := (raw & (1 << 20)) != 0
		rn := uint8((raw >> 16) & 0x0F)
		regList := uint16(raw & 0xFFFF)

		dec.Rn = rn
		dec.RegList = regList
		dec.Imm = uint32(regList)
		dec.HasImm = true

		if rn == isa.SP {
			if lBit {
				dec.Op = isa.OpPOP
			} else {
				dec.Op = isa.OpPUSH
			}
		} else {
			if lBit {
				dec.Op = isa.OpLDM
			} else {
				dec.Op = isa.OpSTM
			}
		}
		return dec, nil
	}

	// 5. Load / Store Extra (Halfword / Signed Byte) (cond | 000 | P | U | I | W | L | Rn | Rd | imm4H | 1 | S | H | 1 | Rm/imm4L)
	if (raw&0x0E000000) == 0x00000000 && (raw&0x00000090) == 0x00000090 {
		// Multiply check: MUL/MLA also have 1001 at bits 7..4
		isMul := (raw & 0x0FC000F0) == 0x00000090
		isMla := (raw & 0x0FC000F0) == 0x00200090
		if isMul {
			dec.Op = isa.OpMUL
			dec.SetFlags = (raw & (1 << 20)) != 0
			dec.Rd = uint8((raw >> 16) & 0x0F)
			dec.Rm = uint8(raw & 0x0F)
			dec.Rn = uint8((raw >> 8) & 0x0F)
			return dec, nil
		}
		if isMla {
			dec.Op = isa.OpMLA
			dec.SetFlags = (raw & (1 << 20)) != 0
			dec.Rd = uint8((raw >> 16) & 0x0F)
			dec.Rm = uint8(raw & 0x0F)
			dec.Rn = uint8((raw >> 8) & 0x0F)
			// Src2 is Rn in ARM MLA
			return dec, nil
		}

		lBit := (raw & (1 << 20)) != 0
		sBit := (raw & (1 << 6)) != 0
		hBit := (raw & (1 << 5)) != 0

		dec.Rn = uint8((raw >> 16) & 0x0F)
		dec.Rd = uint8((raw >> 12) & 0x0F)

		uBit := (raw & (1 << 23)) != 0
		iBit := (raw & (1 << 22)) != 0

		var offset uint32
		if iBit {
			imm4H := (raw >> 8) & 0x0F
			imm4L := raw & 0x0F
			offset = (imm4H << 4) | imm4L
			dec.HasImm = true
			if !uBit {
				dec.Imm = uint32(-int32(offset))
			} else {
				dec.Imm = offset
			}
		} else {
			dec.Rm = uint8(raw & 0x0F)
		}

		if !sBit && hBit {
			if lBit {
				dec.Op = isa.OpLDRH
			} else {
				dec.Op = isa.OpSTRH
			}
		} else if sBit && !hBit {
			dec.Op = isa.OpLDRSB
		} else if sBit && hBit {
			dec.Op = isa.OpLDRSH
		} else {
			return nil, fmt.Errorf("unsupported load/store extra raw: 0x%08X", raw)
		}
		return dec, nil
	}

	// 6. Load / Store Word and Byte (cond | 01 | I | P | U | B | W | L | Rn | Rd | offset12)
	if (raw & 0x0C000000) == 0x04000000 {
		isRegShift := (raw & (1 << 25)) != 0 // In LDR/STR, bit 25 I=1 means register offset
		bBit := (raw & (1 << 22)) != 0
		lBit := (raw & (1 << 20)) != 0
		uBit := (raw & (1 << 23)) != 0

		dec.Rn = uint8((raw >> 16) & 0x0F)
		dec.Rd = uint8((raw >> 12) & 0x0F)

		if lBit {
			if bBit {
				dec.Op = isa.OpLDRB
			} else {
				dec.Op = isa.OpLDR
			}
		} else {
			if bBit {
				dec.Op = isa.OpSTRB
			} else {
				dec.Op = isa.OpSTR
			}
		}

		if !isRegShift {
			dec.HasImm = true
			offset12 := raw & 0xFFF
			if !uBit {
				dec.Imm = uint32(-int32(offset12))
			} else {
				dec.Imm = offset12
			}
		} else {
			dec.Rm = uint8(raw & 0x0F)
			dec.ShiftType = uint8((raw >> 5) & 0x03)
			dec.ShiftAmount = uint8((raw >> 7) & 0x1F)
		}
		return dec, nil
	}

	// 7. Data Processing (cond | 00 | I | opcode | S | Rn | Rd | operand2)
	if (raw & 0x0C000000) == 0x00000000 {
		iBit := (raw & (1 << 25)) != 0
		opCodeBits := (raw >> 21) & 0x0F
		dec.SetFlags = (raw & (1 << 20)) != 0
		dec.Rn = uint8((raw >> 16) & 0x0F)
		dec.Rd = uint8((raw >> 12) & 0x0F)

		// Map ARM opcode bits to IVM opcode
		switch opCodeBits {
		case 0x00:
			dec.Op = isa.OpAND
		case 0x01:
			dec.Op = isa.OpEOR
		case 0x02:
			dec.Op = isa.OpSUB
		case 0x03:
			dec.Op = isa.OpRSB
		case 0x04:
			dec.Op = isa.OpADD
		case 0x05:
			dec.Op = isa.OpADC
		case 0x06:
			dec.Op = isa.OpSBC
		case 0x08:
			dec.Op = isa.OpTST
			dec.SetFlags = true
		case 0x09:
			dec.Op = isa.OpTEQ
			dec.SetFlags = true
		case 0x0A:
			dec.Op = isa.OpCMP
			dec.SetFlags = true
		case 0x0B:
			dec.Op = isa.OpCMN
			dec.SetFlags = true
		case 0x0C:
			dec.Op = isa.OpORR
		case 0x0D:
			dec.Op = isa.OpMOV
		case 0x0E:
			dec.Op = isa.OpBIC
		case 0x0F:
			dec.Op = isa.OpMVN
		default:
			return nil, fmt.Errorf("unknown ARM data opcode 0x%X in raw 0x%08X", opCodeBits, raw)
		}

		// NOP check (MOV R0, R0)
		if dec.Op == isa.OpMOV && dec.Rd == 0 && dec.Rn == 0 && (raw&0xFFF) == 0 && !dec.SetFlags {
			dec.Op = isa.OpNOP
			return dec, nil
		}

		if iBit {
			dec.HasImm = true
			imm8 := raw & 0xFF
			rot := (raw >> 8) & 0x0F
			dec.Imm = rotateRight(imm8, rot*2)
		} else {
			dec.Rm = uint8(raw & 0x0F)
			dec.ShiftType = uint8((raw >> 5) & 0x03)
			dec.ShiftAmount = uint8((raw >> 7) & 0x1F)
		}
		return dec, nil
	}

	return nil, fmt.Errorf("unhandled ARM32 instruction raw: 0x%08X", raw)
}
