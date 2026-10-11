package vm

import (
	"fmt"
	"math/bits"
	"runtime"
)

// HostBridge manages native execution fallback when the VM encounters an instruction
// that is outside the standard virtualized ISA. It context-switches the VM state to
// host CPU evaluation and restores updated registers and flags.
type HostBridge struct {
	Debug         bool
	FallbackCount uint64
	CustomHandler func(cpu *CPU, mem *Memory, rawInst uint32) (bool, error)
}

func NewHostBridge(debug bool) *HostBridge {
	return &HostBridge{
		Debug: debug,
	}
}

// Execute performs native host CPU execution for an unvirtualized ARM32 instruction.
func (b *HostBridge) Execute(cpu *CPU, mem *Memory, rawInst uint32, currPC uint32) error {
	b.FallbackCount++

	if b.Debug {
		fmt.Printf("[Host CPU Fallback] PC: 0x%08X | Raw Inst: 0x%08X | Host Arch: %s\n",
			currPC, rawInst, runtime.GOARCH)
	}

	// 1. Check custom handler hook if registered
	if b.CustomHandler != nil {
		handled, err := b.CustomHandler(cpu, mem, rawInst)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}

	if cpu.Arch == 0x0002 { // ARMv8 (64-bit)
		return b.executeARMv8Fallback(cpu, mem, rawInst, currPC)
	}

	// 2. 64-bit Long Multiply Instructions (UMULL, SMULL, UMLAL, SMLAL)
	// Mask: cond | 0000 1xxx | S | RdHi | RdLo | Rs | 1001 | Rm
	if (rawInst & 0x0F8000F0) == 0x00800090 {
		return b.executeLongMultiply(cpu, rawInst)
	}

	// 3. Bit Extension Instructions (UXTB, UXTH, SXTB, SXTH)
	// Mask: cond | 0110 1xxx | xxxx | Rd | rot | 0111 | Rm
	if (rawInst & 0x0F800070) == 0x06800070 {
		return b.executeExtend(cpu, rawInst)
	}

	// 4. Endian Conversion Instructions (REV, REV16, REVSH)
	if (rawInst & 0x0FBF0FF0) == 0x06BF0F10 { // REV
		rd := uint8((rawInst >> 12) & 0x0F)
		rm := uint8(rawInst & 0x0F)
		val := cpu.GetReg(rm)
		reversed := bits.ReverseBytes32(val)
		cpu.SetReg(rd, reversed)
		return nil
	}
	if (rawInst & 0x0FBF0FF0) == 0x06BF0FB0 { // REV16
		rd := uint8((rawInst >> 12) & 0x0F)
		rm := uint8(rawInst & 0x0F)
		val := cpu.GetReg(rm)
		hi := bits.ReverseBytes16(uint16(val >> 16))
		lo := bits.ReverseBytes16(uint16(val & 0xFFFF))
		cpu.SetReg(rd, (uint32(hi)<<16)|uint32(lo))
		return nil
	}
	if (rawInst & 0x0FBF0FF0) == 0x06FF0FB0 { // REVSH
		rd := uint8((rawInst >> 12) & 0x0F)
		rm := uint8(rawInst & 0x0F)
		val := cpu.GetReg(rm)
		lo := bits.ReverseBytes16(uint16(val & 0xFFFF))
		signedVal := int32(int16(lo))
		cpu.SetReg(rd, uint32(signedVal))
		return nil
	}

	// 5. Count Leading Zeros (CLZ) - cond | 0001 0110 | 1111 | Rd | 1111 | 0001 | Rm
	if (rawInst & 0x0FFF0FF0) == 0x016F0F10 {
		rd := uint8((rawInst >> 12) & 0x0F)
		rm := uint8(rawInst & 0x0F)
		val := cpu.GetReg(rm)
		lz := uint32(bits.LeadingZeros32(val))
		cpu.SetReg(rd, lz)
		return nil
	}

	// 6. Bitfield Manipulation Instructions (UBFX, SBFX, BFI, BFC)
	if (rawInst & 0x0FB00070) == 0x07E00050 { // UBFX
		rd := uint8((rawInst >> 12) & 0x0F)
		rn := uint8(rawInst & 0x0F)
		lsb := (rawInst >> 7) & 0x1F
		widthMinus1 := (rawInst >> 16) & 0x1F
		width := widthMinus1 + 1

		val := cpu.GetReg(rn)
		extracted := (val >> lsb) & ((1 << width) - 1)
		cpu.SetReg(rd, extracted)
		return nil
	}
	if (rawInst & 0x0FB00070) == 0x07A00050 { // SBFX
		rd := uint8((rawInst >> 12) & 0x0F)
		rn := uint8(rawInst & 0x0F)
		lsb := (rawInst >> 7) & 0x1F
		widthMinus1 := (rawInst >> 16) & 0x1F
		width := widthMinus1 + 1

		val := cpu.GetReg(rn)
		extracted := (val >> lsb) & ((1 << width) - 1)
		// Sign extend from bit (width - 1)
		signBit := uint32(1 << (width - 1))
		if (extracted & signBit) != 0 {
			extracted |= ^((1 << width) - 1)
		}
		cpu.SetReg(rd, extracted)
		return nil
	}
	if (rawInst & 0x0FB00070) == 0x07C00010 { // BFI / BFC
		rd := uint8((rawInst >> 12) & 0x0F)
		rn := uint8(rawInst & 0x0F)
		lsb := (rawInst >> 7) & 0x1F
		msb := (rawInst >> 16) & 0x1F

		if msb >= lsb {
			width := msb - lsb + 1
			mask := ((uint32(1) << width) - 1) << lsb

			dstVal := cpu.GetReg(rd) & (^mask)
			if rn != 15 { // BFI
				srcVal := (cpu.GetReg(rn) & ((1 << width) - 1)) << lsb
				cpu.SetReg(rd, dstVal|srcVal)
			} else { // BFC (Clear)
				cpu.SetReg(rd, dstVal)
			}
		}
		return nil
	}

	// 7. Hardware Division (SDIV, UDIV)
	if (rawInst & 0x0FF0F0F0) == 0x0710F010 { // SDIV
		rd := uint8((rawInst >> 16) & 0x0F)
		rn := uint8(rawInst & 0x0F)
		rm := uint8((rawInst >> 8) & 0x0F)
		n := int32(cpu.GetReg(rn))
		m := int32(cpu.GetReg(rm))
		if m != 0 {
			cpu.SetReg(rd, uint32(n/m))
		} else {
			cpu.SetReg(rd, 0)
		}
		return nil
	}
	if (rawInst & 0x0FF0F0F0) == 0x0730F010 { // UDIV
		rd := uint8((rawInst >> 16) & 0x0F)
		rn := uint8(rawInst & 0x0F)
		rm := uint8((rawInst >> 8) & 0x0F)
		n := cpu.GetReg(rn)
		m := cpu.GetReg(rm)
		if m != 0 {
			cpu.SetReg(rd, n/m)
		} else {
			cpu.SetReg(rd, 0)
		}
		return nil
	}

	// 8. Block Transfer (LDM/STM) for arbitrary base registers
	if (rawInst & 0x0E000000) == 0x08000000 {
		return b.executeBlockTransfer(cpu, mem, rawInst)
	}

	// 9. Memory Barriers (DMB, DSB, ISB) and NOP hints
	if (rawInst&0x0F000000) == 0x0F000000 || (rawInst&0x0FFFF000) == 0x057FF000 {
		// Treated as host NOP / synchronization barrier
		return nil
	}

	return fmt.Errorf("host CPU cannot execute unmapped instruction 0x%08X (PC: 0x%08X)", rawInst, currPC)
}

func (b *HostBridge) executeLongMultiply(cpu *CPU, raw uint32) error {
	uBit := (raw & (1 << 22)) != 0 // 1 = Signed (SMULL/SMLAL), 0 = Unsigned (UMULL/UMLAL)
	aBit := (raw & (1 << 21)) != 0 // 1 = Accumulate (UMLAL/SMLAL), 0 = Multiply (UMULL/SMULL)
	sBit := (raw & (1 << 20)) != 0 // Set condition flags

	rdHi := uint8((raw >> 16) & 0x0F)
	rdLo := uint8((raw >> 12) & 0x0F)
	rs := uint8((raw >> 8) & 0x0F)
	rm := uint8(raw & 0x0F)

	valRs := cpu.GetReg(rs)
	valRm := cpu.GetReg(rm)

	var res64 uint64

	if uBit { // Signed Multiply
		signedProd := int64(int32(valRs)) * int64(int32(valRm))
		if aBit { // SMLAL
			acc := int64(uint64(cpu.GetReg(rdLo)) | (uint64(cpu.GetReg(rdHi)) << 32))
			signedProd += acc
		}
		res64 = uint64(signedProd)
	} else { // Unsigned Multiply
		unsignedProd := uint64(valRs) * uint64(valRm)
		if aBit { // UMLAL
			acc := uint64(cpu.GetReg(rdLo)) | (uint64(cpu.GetReg(rdHi)) << 32)
			unsignedProd += acc
		}
		res64 = unsignedProd
	}

	cpu.SetReg(rdLo, uint32(res64&0xFFFFFFFF))
	cpu.SetReg(rdHi, uint32(res64>>32))

	if sBit {
		cpu.FlagZ = (res64 == 0)
		cpu.FlagN = (int64(res64) < 0)
	}

	return nil
}

func (b *HostBridge) executeExtend(cpu *CPU, raw uint32) error {
	op := (raw >> 20) & 0x07
	rd := uint8((raw >> 12) & 0x0F)
	rm := uint8(raw & 0x0F)
	rot := ((raw >> 10) & 0x03) * 8

	val := cpu.GetReg(rm)
	if rot > 0 {
		val = (val >> rot) | (val << (32 - rot))
	}

	switch op {
	case 0x06: // UXTB
		cpu.SetReg(rd, val&0xFF)
	case 0x07: // UXTH
		cpu.SetReg(rd, val&0xFFFF)
	case 0x02: // SXTB
		cpu.SetReg(rd, uint32(int32(int8(val&0xFF))))
	case 0x03: // SXTH
		cpu.SetReg(rd, uint32(int32(int16(val&0xFFFF))))
	default:
		return fmt.Errorf("unsupported extend opcode 0x%X in raw 0x%08X", op, raw)
	}
	return nil
}

func (b *HostBridge) executeBlockTransfer(cpu *CPU, mem *Memory, raw uint32) error {
	pBit := (raw & (1 << 24)) != 0 // Pre/Post index
	uBit := (raw & (1 << 23)) != 0 // Up/Down
	wBit := (raw & (1 << 21)) != 0 // Write-back
	lBit := (raw & (1 << 20)) != 0 // Load / Store
	rn := uint8((raw >> 16) & 0x0F)
	regList := uint16(raw & 0xFFFF)

	baseAddr := cpu.GetReg(rn)
	currAddr := baseAddr

	count := bits.OnesCount16(regList)
	totalBytes := uint32(count * 4)

	var startAddr uint32
	if uBit {
		if pBit {
			startAddr = currAddr + 4
		} else {
			startAddr = currAddr
		}
	} else {
		if pBit {
			startAddr = currAddr - totalBytes
		} else {
			startAddr = currAddr - totalBytes + 4
		}
	}

	stepAddr := startAddr
	for r := 0; r <= 15; r++ {
		if (regList & (1 << uint(r))) != 0 {
			if lBit { // LDM
				val := mem.Read32(stepAddr)
				cpu.SetReg(uint8(r), val)
			} else { // STM
				val := cpu.GetReg(uint8(r))
				mem.Write32(stepAddr, val)
			}
			stepAddr += 4
		}
	}

	if wBit {
		if uBit {
			cpu.SetReg(rn, baseAddr+totalBytes)
		} else {
			cpu.SetReg(rn, baseAddr-totalBytes)
		}
	}

	return nil
}

func (b *HostBridge) executeARMv8Fallback(cpu *CPU, mem *Memory, raw uint32, currPC uint32) error {
	// 1. SDIV / UDIV (Integer Division)
	if (raw & 0x7FE0FFE0) == 0x1AC00C00 { // SDIV
		is64 := (raw & (1 << 31)) != 0
		rd := uint8(raw & 0x1F)
		rn := uint8((raw >> 5) & 0x1F)
		rm := uint8((raw >> 16) & 0x1F)
		if is64 {
			d := int64(cpu.GetReg64(rm))
			if d == 0 {
				cpu.SetReg64(rd, 0)
			} else {
				cpu.SetReg64(rd, uint64(int64(cpu.GetReg64(rn))/d))
			}
		} else {
			d := int32(cpu.GetReg(rm))
			if d == 0 {
				cpu.SetReg(rd, 0)
			} else {
				cpu.SetReg(rd, uint32(int32(cpu.GetReg(rn))/d))
			}
		}
		return nil
	}
	if (raw & 0x7FE0FFE0) == 0x1AC00800 { // UDIV
		is64 := (raw & (1 << 31)) != 0
		rd := uint8(raw & 0x1F)
		rn := uint8((raw >> 5) & 0x1F)
		rm := uint8((raw >> 16) & 0x1F)
		if is64 {
			d := cpu.GetReg64(rm)
			if d == 0 {
				cpu.SetReg64(rd, 0)
			} else {
				cpu.SetReg64(rd, cpu.GetReg64(rn)/d)
			}
		} else {
			d := cpu.GetReg(rm)
			if d == 0 {
				cpu.SetReg(rd, 0)
			} else {
				cpu.SetReg(rd, cpu.GetReg(rn)/d)
			}
		}
		return nil
	}

	// 2. MADD / MSUB (Multiply-Add / Multiply-Subtract)
	if (raw & 0x7FE08000) == 0x1B000000 {
		is64 := (raw & (1 << 31)) != 0
		isSub := (raw & (1 << 15)) != 0
		rd := uint8(raw & 0x1F)
		rn := uint8((raw >> 5) & 0x1F)
		rm := uint8((raw >> 16) & 0x1F)
		ra := uint8((raw >> 10) & 0x1F)
		if is64 {
			acc := cpu.GetReg64(ra)
			prod := cpu.GetReg64(rn) * cpu.GetReg64(rm)
			if isSub {
				cpu.SetReg64(rd, acc-prod)
			} else {
				cpu.SetReg64(rd, acc+prod)
			}
		} else {
			acc := cpu.GetReg(ra)
			prod := cpu.GetReg(rn) * cpu.GetReg(rm)
			if isSub {
				cpu.SetReg(rd, acc-prod)
			} else {
				cpu.SetReg(rd, acc+prod)
			}
		}
		return nil
	}

	// 3. Count Leading Zeros (CLZ)
	if (raw & 0x7FE0FC00) == 0x5AC01000 {
		is64 := (raw & (1 << 31)) != 0
		rd := uint8(raw & 0x1F)
		rn := uint8((raw >> 5) & 0x1F)
		if is64 {
			cpu.SetReg64(rd, uint64(bits.LeadingZeros64(cpu.GetReg64(rn))))
		} else {
			cpu.SetReg(rd, uint32(bits.LeadingZeros32(cpu.GetReg(rn))))
		}
		return nil
	}

	// 4. Byte Reversal (REV)
	if (raw & 0x7FE0FC00) == 0x5AC00800 {
		is64 := (raw & (1 << 31)) != 0
		rd := uint8(raw & 0x1F)
		rn := uint8((raw >> 5) & 0x1F)
		if is64 {
			cpu.SetReg64(rd, bits.ReverseBytes64(cpu.GetReg64(rn)))
		} else {
			cpu.SetReg(rd, bits.ReverseBytes32(cpu.GetReg(rn)))
		}
		return nil
	}

	// Memory Barriers (DMB, DSB, ISB) and NOP hints
	if (raw & 0xFFFFF000) == 0xD5033000 {
		return nil
	}

	return fmt.Errorf("host CPU cannot execute unmapped ARMv8 instruction 0x%08X (PC: 0x%08X)", raw, currPC)
}
