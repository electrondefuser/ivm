package vm

import (
	"fmt"
	"ivm/src/isa"
)

type CPU struct {
	Regs     [16]uint32
	FlagN    bool // Negative flag
	FlagZ    bool // Zero flag
	FlagC    bool // Carry flag
	FlagV    bool // Overflow flag
	Halted   bool
	ExitCode int
}

func NewCPU() *CPU {
	return &CPU{}
}

func (cpu *CPU) GetReg(r uint8) uint32 {
	if r > 15 {
		return 0
	}
	return cpu.Regs[r]
}

func (cpu *CPU) SetReg(r uint8, val uint32) {
	if r <= 15 {
		cpu.Regs[r] = val
	}
}

// CheckCondition evaluates the ARM/IVM condition code against current CPSR flags
func (cpu *CPU) CheckCondition(cond uint8) bool {
	switch cond {
	case isa.CondEQ:
		return cpu.FlagZ
	case isa.CondNE:
		return !cpu.FlagZ
	case isa.CondCS:
		return cpu.FlagC
	case isa.CondCC:
		return !cpu.FlagC
	case isa.CondMI:
		return cpu.FlagN
	case isa.CondPL:
		return !cpu.FlagN
	case isa.CondVS:
		return cpu.FlagV
	case isa.CondVC:
		return !cpu.FlagV
	case isa.CondHI:
		return cpu.FlagC && !cpu.FlagZ
	case isa.CondLS:
		return !cpu.FlagC || cpu.FlagZ
	case isa.CondGE:
		return cpu.FlagN == cpu.FlagV
	case isa.CondLT:
		return cpu.FlagN != cpu.FlagV
	case isa.CondGT:
		return !cpu.FlagZ && (cpu.FlagN == cpu.FlagV)
	case isa.CondLE:
		return cpu.FlagZ || (cpu.FlagN != cpu.FlagV)
	case isa.CondAL, isa.CondNV:
		return true
	default:
		return true
	}
}

// UpdateFlagsNZ updates Zero and Negative flags based on 32-bit result
func (cpu *CPU) UpdateFlagsNZ(res uint32) {
	cpu.FlagZ = (res == 0)
	cpu.FlagN = (int32(res) < 0)
}

// UpdateAddFlags updates N, Z, C, V flags for Addition (a + b + carryIn)
func (cpu *CPU) UpdateAddFlags(a, b, carryIn uint32) uint32 {
	res64 := uint64(a) + uint64(b) + uint64(carryIn)
	res32 := uint32(res64)

	cpu.FlagZ = (res32 == 0)
	cpu.FlagN = (int32(res32) < 0)
	cpu.FlagC = (res64 > 0xFFFFFFFF)

	// Overflow occurs if both operands have same sign, but result has different sign
	aSign := (a >> 31) & 1
	bSign := (b >> 31) & 1
	rSign := (res32 >> 31) & 1
	cpu.FlagV = (aSign == bSign) && (aSign != rSign)

	return res32
}

// UpdateSubFlags updates N, Z, C, V flags for Subtraction (a - b - borrow)
func (cpu *CPU) UpdateSubFlags(a, b, borrowIn uint32) uint32 {
	res64 := uint64(a) - uint64(b) - uint64(borrowIn)
	res32 := uint32(res64)

	cpu.FlagZ = (res32 == 0)
	cpu.FlagN = (int32(res32) < 0)
	cpu.FlagC = (a >= (b + borrowIn)) // ARM carry flag is NOT borrow (C=1 if no borrow)

	aSign := (a >> 31) & 1
	bSign := (b >> 31) & 1
	rSign := (res32 >> 31) & 1
	cpu.FlagV = (aSign != bSign) && (aSign != rSign)

	return res32
}

// DumpState returns a string representation of CPU state
func (cpu *CPU) DumpState() string {
	return fmt.Sprintf(
		"R0:  0x%08X  R1:  0x%08X  R2:  0x%08X  R3:  0x%08X\n"+
			"R4:  0x%08X  R5:  0x%08X  R6:  0x%08X  R7:  0x%08X\n"+
			"R8:  0x%08X  R9:  0x%08X  R10: 0x%08X  R11: 0x%08X\n"+
			"R12: 0x%08X  SP:  0x%08X  LR:  0x%08X  PC:  0x%08X\n"+
			"Flags: [N=%t Z=%t C=%t V=%t]",
		cpu.Regs[0], cpu.Regs[1], cpu.Regs[2], cpu.Regs[3],
		cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7],
		cpu.Regs[8], cpu.Regs[9], cpu.Regs[10], cpu.Regs[11],
		cpu.Regs[12], cpu.Regs[isa.SP], cpu.Regs[isa.LR], cpu.Regs[isa.PC],
		cpu.FlagN, cpu.FlagZ, cpu.FlagC, cpu.FlagV,
	)
}
