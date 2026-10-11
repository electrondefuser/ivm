package vm

import (
	"encoding/binary"
	"fmt"
)

// EncodeInstruction encodes an Instruction into an 8-byte slice.
func EncodeInstruction(inst Instruction) [8]byte {
	var buf [8]byte

	buf[0] = uint8(inst.Op)

	var flags uint8
	flags |= (inst.Cond & 0x0F) << 4
	if inst.SetFlags {
		flags |= (1 << 3)
	}
	if inst.HasImm {
		flags |= (1 << 2)
	}
	flags |= (inst.ShiftType & 0x03)
	buf[1] = flags

	if inst.Arch == ArchARMv8 || inst.Is64Bit || inst.RegDst > 15 || inst.RegSrc1 > 15 || inst.RegSrc2 > 15 {
		// ARMv8 16-bit register packing:
		// Bits 0..4:   RegDst (5 bits: 0-31)
		// Bits 5..9:   RegSrc1 (5 bits: 0-31)
		// Bits 10..14: RegSrc2 (5 bits: 0-31)
		// Bit 15:      Is64Bit (1 bit: 1 if 64-bit, 0 if 32-bit)
		regVal := uint16(inst.RegDst&0x1F) |
			(uint16(inst.RegSrc1&0x1F) << 5) |
			(uint16(inst.RegSrc2&0x1F) << 10)
		if inst.Is64Bit {
			regVal |= (1 << 15)
		}
		binary.LittleEndian.PutUint16(buf[2:4], regVal)
	} else {
		// ARMv7 standard 4-bit register packing:
		buf[2] = ((inst.RegDst & 0x0F) << 4) | (inst.RegSrc1 & 0x0F)
		buf[3] = ((inst.RegSrc2 & 0x0F) << 4) | (inst.ShiftAmount & 0x0F)
	}

	binary.LittleEndian.PutUint32(buf[4:8], inst.Imm)
	return buf
}

/* EncodeSlice encodes a list of Instructions into a byte buffer. */
func EncodeSlice(instructions []Instruction) []byte {
	out := make([]byte, len(instructions)*8)

	for i, inst := range instructions {
		encoded := EncodeInstruction(inst)
		copy(out[i*8:(i+1)*8], encoded[:])
	}
	return out
}

/* FormatReg returns string representation of ARM/IVM register */
func FormatReg(r uint8) string {
	switch r {
	case SP:
		return "sp"
	case LR:
		return "lr"
	case PC:
		return "pc"
	default:
		if r <= 12 {
			return fmt.Sprintf("r%d", r)
		}
		return fmt.Sprintf("r%d", r)
	}
}
