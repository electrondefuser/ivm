package vm

import (
	"fmt"
)

type CPU struct {
	Regs     [32]uint64
	PC       uint64 // Independent Program Counter (especially for ARMv8)
	FlagN    bool   // Negative flag
	FlagZ    bool   // Zero flag
	FlagC    bool   // Carry flag
	FlagV    bool   // Overflow flag
	Halted   bool
	ExitCode int
	Arch     uint16 // format.ArchARMv7 (0x01) or format.ArchARMv8 (0x02)
}

func NewCPU() *CPU {
	return &CPU{}
}

func (cpu *CPU) GetPC() uint32 {
	if cpu.Arch == 0x0002 { // ARMv8 (64-bit): independent PC
		return uint32(cpu.PC)
	}
	// ARMv7 (or uninitialized default): PC is R15
	return uint32(cpu.Regs[PC])
}

func (cpu *CPU) SetPC(val uint32) {
	cpu.PC = uint64(val)
	if cpu.Arch != 0x0002 {
		cpu.Regs[PC] = uint64(val)
	}
}

func (cpu *CPU) GetReg(r uint8) uint32 {
	if r >= 32 {
		return 0
	}
	return uint32(cpu.Regs[r])
}

func (cpu *CPU) SetReg(r uint8, val uint32) {
	if r < 32 {
		cpu.Regs[r] = uint64(val)
		if r == PC && cpu.Arch != 0x0002 {
			cpu.PC = uint64(val)
		}
	}
}

func (cpu *CPU) GetReg64(r uint8) uint64 {
	if r >= 32 {
		return 0
	}
	return cpu.Regs[r]
}

func (cpu *CPU) SetReg64(r uint8, val uint64) {
	if r < 32 {
		cpu.Regs[r] = val
	}
}

// CheckCondition evaluates the ARM condition code against current CPSR flags
func (cpu *CPU) CheckCondition(cond uint8) bool {
	switch cond {
	case CondEQ:
		return cpu.FlagZ
	case CondNE:
		return !cpu.FlagZ
	case CondCS:
		return cpu.FlagC
	case CondCC:
		return !cpu.FlagC
	case CondMI:
		return cpu.FlagN
	case CondPL:
		return !cpu.FlagN
	case CondVS:
		return cpu.FlagV
	case CondVC:
		return !cpu.FlagV
	case CondHI:
		return cpu.FlagC && !cpu.FlagZ
	case CondLS:
		return !cpu.FlagC || cpu.FlagZ
	case CondGE:
		return cpu.FlagN == cpu.FlagV
	case CondLT:
		return cpu.FlagN != cpu.FlagV
	case CondGT:
		return !cpu.FlagZ && (cpu.FlagN == cpu.FlagV)
	case CondLE:
		return cpu.FlagZ || (cpu.FlagN != cpu.FlagV)
	case CondAL, CondNV:
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

// UpdateFlagsNZ64 updates Zero and Negative flags based on 64-bit result
func (cpu *CPU) UpdateFlagsNZ64(res uint64) {
	cpu.FlagZ = (res == 0)
	cpu.FlagN = (int64(res) < 0)
}

// UpdateAddFlags updates N, Z, C, V flags for 32-bit Addition (a + b + carryIn)
func (cpu *CPU) UpdateAddFlags(a, b, carryIn uint32) uint32 {
	res64 := uint64(a) + uint64(b) + uint64(carryIn)
	res32 := uint32(res64)

	cpu.FlagZ = (res32 == 0)
	cpu.FlagN = (int32(res32) < 0)
	cpu.FlagC = (res64 > 0xFFFFFFFF)

	aSign := (a >> 31) & 1
	bSign := (b >> 31) & 1
	rSign := (res32 >> 31) & 1
	cpu.FlagV = (aSign == bSign) && (aSign != rSign)

	return res32
}

// UpdateSubFlags updates N, Z, C, V flags for 32-bit Subtraction (a - b - borrow)
func (cpu *CPU) UpdateSubFlags(a, b, borrowIn uint32) uint32 {
	res64 := uint64(a) - uint64(b) - uint64(borrowIn)
	res32 := uint32(res64)

	cpu.FlagZ = (res32 == 0)
	cpu.FlagN = (int32(res32) < 0)
	cpu.FlagC = (a >= (b + borrowIn))

	aSign := (a >> 31) & 1
	bSign := (b >> 31) & 1
	rSign := (res32 >> 31) & 1
	cpu.FlagV = (aSign != bSign) && (aSign != rSign)

	return res32
}

// UpdateAddFlags64 updates N, Z, C, V flags for 64-bit Addition
func (cpu *CPU) UpdateAddFlags64(a, b, carryIn uint64) uint64 {
	res := a + b + carryIn

	cpu.FlagZ = (res == 0)
	cpu.FlagN = (int64(res) < 0)
	cpu.FlagC = (res < a) || (carryIn > 0 && res <= a)

	aSign := (a >> 63) & 1
	bSign := (b >> 63) & 1
	rSign := (res >> 63) & 1
	cpu.FlagV = (aSign == bSign) && (aSign != rSign)

	return res
}

// UpdateSubFlags64 updates N, Z, C, V flags for 64-bit Subtraction
func (cpu *CPU) UpdateSubFlags64(a, b, borrowIn uint64) uint64 {
	res := a - b - borrowIn

	cpu.FlagZ = (res == 0)
	cpu.FlagN = (int64(res) < 0)
	cpu.FlagC = (a >= (b + borrowIn))

	aSign := (a >> 63) & 1
	bSign := (b >> 63) & 1
	rSign := (res >> 63) & 1
	cpu.FlagV = (aSign != bSign) && (aSign != rSign)

	return res
}

// DumpState returns a string representation of CPU state
func (cpu *CPU) DumpState() string {
	if cpu.Arch == 0x0002 { // ARMv8 (64-bit)
		return fmt.Sprintf(
			"X0:  0x%016X  X1:  0x%016X  X2:  0x%016X  X3:  0x%016X\n"+
				"X4:  0x%016X  X5:  0x%016X  X6:  0x%016X  X7:  0x%016X\n"+
				"X8:  0x%016X  X9:  0x%016X  X10: 0x%016X  X11: 0x%016X\n"+
				"X12: 0x%016X  X13: 0x%016X  X14: 0x%016X  X15: 0x%016X\n"+
				"X16: 0x%016X  X17: 0x%016X  X18: 0x%016X  X19: 0x%016X\n"+
				"X20: 0x%016X  X21: 0x%016X  X22: 0x%016X  X23: 0x%016X\n"+
				"X24: 0x%016X  X25: 0x%016X  X26: 0x%016X  X27: 0x%016X\n"+
				"X28: 0x%016X  FP:  0x%016X  LR:  0x%016X  SP:  0x%016X\n"+
				"PC:  0x%016X  Flags: [N=%t Z=%t C=%t V=%t]",
			cpu.Regs[0], cpu.Regs[1], cpu.Regs[2], cpu.Regs[3],
			cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7],
			cpu.Regs[8], cpu.Regs[9], cpu.Regs[10], cpu.Regs[11],
			cpu.Regs[12], cpu.Regs[13], cpu.Regs[14], cpu.Regs[15],
			cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19],
			cpu.Regs[20], cpu.Regs[21], cpu.Regs[22], cpu.Regs[23],
			cpu.Regs[24], cpu.Regs[25], cpu.Regs[26], cpu.Regs[27],
			cpu.Regs[28], cpu.Regs[29], cpu.Regs[30], cpu.Regs[31],
			cpu.PC, cpu.FlagN, cpu.FlagZ, cpu.FlagC, cpu.FlagV,
		)
	}

	// ARMv7 (32-bit default)
	return fmt.Sprintf(
		"R0:  0x%08X  R1:  0x%08X  R2:  0x%08X  R3:  0x%08X\n"+
			"R4:  0x%08X  R5:  0x%08X  R6:  0x%08X  R7:  0x%08X\n"+
			"R8:  0x%08X  R9:  0x%08X  R10: 0x%08X  R11: 0x%08X\n"+
			"R12: 0x%08X  SP:  0x%08X  LR:  0x%08X  PC:  0x%08X\n"+
			"Flags: [N=%t Z=%t C=%t V=%t]",
		uint32(cpu.Regs[0]), uint32(cpu.Regs[1]), uint32(cpu.Regs[2]), uint32(cpu.Regs[3]),
		uint32(cpu.Regs[4]), uint32(cpu.Regs[5]), uint32(cpu.Regs[6]), uint32(cpu.Regs[7]),
		uint32(cpu.Regs[8]), uint32(cpu.Regs[9]), uint32(cpu.Regs[10]), uint32(cpu.Regs[11]),
		uint32(cpu.Regs[12]), uint32(cpu.Regs[SP]), uint32(cpu.Regs[LR]), uint32(cpu.Regs[PC]),
		cpu.FlagN, cpu.FlagZ, cpu.FlagC, cpu.FlagV,
	)
}
