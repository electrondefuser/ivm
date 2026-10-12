package vm

import (
	"fmt"
)

type Interpreter struct {
	CPU               *CPU
	Memory            *Memory
	InstructionMap    map[uint32]Instruction
	HostBridge        *HostBridge
	HostSyscallBridge *HostSyscallBridge
	Debug             bool
	InstructionCount  uint64
}

func NewInterpreter(debug bool) *Interpreter {
	return &Interpreter{
		CPU:               NewCPU(),
		Memory:            NewMemory(),
		InstructionMap:    make(map[uint32]Instruction),
		HostBridge:        NewHostBridge(debug),
		HostSyscallBridge: NewHostSyscallBridge(debug),
		Debug:             debug,
	}
}

/* EvaluateShift computes operand shifted by shiftType and shiftAmount */
func EvaluateShift(val uint32, shiftType uint8, amount uint8) uint32 {
	if amount == 0 {
		return val
	}
	amount &= 31
	switch shiftType {
	case ShiftLSL:
		return val << amount
	case ShiftLSR:
		return val >> amount
	case ShiftASR:
		return uint32(int32(val) >> amount)
	case ShiftROR:
		return (val >> amount) | (val << (32 - amount))
	default:
		return val
	}
}

/* LoadIVMFile initializes virtual memory, CPU registers, and instruction map from IVMFile */
func (interp *Interpreter) LoadIVMFile(ivmFile *IVMFile) error {
	instructions, err := DecodeSlice(ivmFile.Bytecode, ivmFile.Header.Arch)
	if err != nil {
		return fmt.Errorf("failed to decode bytecode payload: %w", err)
	}

	codeBase := ivmFile.Header.CodeLoadAddr

	// Map each instruction to its 4-byte ARM virtual address slot
	for i, inst := range instructions {
		instAddr := codeBase + uint32(i*4)
		interp.InstructionMap[instAddr] = inst
	}

	// Load raw code image into virtual memory for PC-relative loads and literal pool access
	if len(ivmFile.RawCode) > 0 {
		interp.Memory.LoadSegment(codeBase, ivmFile.RawCode)
	}

	// Load data segment
	if len(ivmFile.Data) > 0 {
		interp.Memory.LoadSegment(ivmFile.Header.DataLoadAddr, ivmFile.Data)
	}

	interp.CPU.Arch = ivmFile.Header.Arch

	// Setup stack (default SP at 0x7FFFFF00)
	spAddr := uint32(0x7FFFFF00)
	if ivmFile.Header.Arch == ArchARMv8 {
		interp.CPU.SetReg64(31, uint64(spAddr)) // X31 is SP in ARMv8
	} else {
		interp.CPU.SetReg(SP, spAddr)
	}

	// Set PC to entry point
	interp.CPU.SetPC(ivmFile.Header.EntryPC)

	return nil
}

/* Step executes a single IVM instruction at CPU.PC */
/* Step executes a single IVM instruction at CPU.PC */
func (interp *Interpreter) Step() error {
	if interp.CPU.Halted {
		return nil
	}

	pc := interp.CPU.GetPC()
	inst, exists := interp.InstructionMap[pc]

	if !exists {
		return fmt.Errorf("execution error: no instruction mapped at PC 0x%08X\n%s", pc, interp.CPU.DumpState())
	}

	// Advance PC by 4 to point to next instruction by default
	nextPC := pc + 4
	interp.CPU.SetPC(nextPC)
	interp.InstructionCount++

	// Check condition code
	if !interp.CPU.CheckCondition(inst.Cond) {
		return nil
	}

	if interp.CPU.Arch == ArchARMv8 {
		return interp.stepARMv8(inst, pc, nextPC)
	}
	return interp.stepARMv7(inst, pc, nextPC)
}

func (interp *Interpreter) stepARMv7(inst Instruction, pc, nextPC uint32) error {
	// Helper to get register value with ARM PC (+8) pipeline offset
	getRegValue := func(r uint8) uint32 {
		if r == PC {
			return pc + 8
		}
		return interp.CPU.GetReg(r)
	}

	// Compute Second Operand (Immediate or Shifted Register)
	getOperand2 := func() uint32 {
		if inst.HasImm {
			return inst.Imm
		}
		rmVal := getRegValue(inst.RegSrc2)
		return EvaluateShift(rmVal, inst.ShiftType, inst.ShiftAmount)
	}

	switch inst.Op {
	case OpNOP:
		// No operation

	case OpMOV:
		op2 := getOperand2()
		interp.CPU.SetReg(inst.RegDst, op2)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(op2)
		}

	case OpMVN:
		op2 := getOperand2()
		res := ^op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpADD:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateAddFlags(op1, op2, 0)
		} else {
			res = op1 + op2
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case OpADC:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		carryIn := uint32(0)
		if interp.CPU.FlagC {
			carryIn = 1
		}
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateAddFlags(op1, op2, carryIn)
		} else {
			res = op1 + op2 + carryIn
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case OpSUB:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateSubFlags(op1, op2, 0)
		} else {
			res = op1 - op2
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case OpSBC:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		borrowIn := uint32(1)
		if interp.CPU.FlagC {
			borrowIn = 0
		}
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateSubFlags(op1, op2, borrowIn)
		} else {
			res = op1 - op2 - borrowIn
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case OpRSB:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateSubFlags(op2, op1, 0)
		} else {
			res = op2 - op1
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case OpMUL:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getRegValue(inst.RegSrc2)
		res := op1 * op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpMLA:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getRegValue(inst.RegSrc2)
		acc := interp.CPU.GetReg(inst.RegDst)
		res := (op1 * op2) + acc
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpAND:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpORR:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 | op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpEOR:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 ^ op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpBIC:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & (^op2)
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpCMP:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		interp.CPU.UpdateSubFlags(op1, op2, 0)

	case OpCMN:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		interp.CPU.UpdateAddFlags(op1, op2, 0)

	case OpTST:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & op2
		interp.CPU.UpdateFlagsNZ(res)

	case OpTEQ:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 ^ op2
		interp.CPU.UpdateFlagsNZ(res)

	case OpLDR:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := interp.Memory.Read32(effAddr)
		interp.CPU.SetReg(inst.RegDst, val)

	case OpSTR:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := getRegValue(inst.RegDst)
		interp.Memory.Write32(effAddr, val)

	case OpLDRB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint32(interp.Memory.Read8(effAddr))
		interp.CPU.SetReg(inst.RegDst, val)

	case OpSTRB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint8(getRegValue(inst.RegDst) & 0xFF)
		interp.Memory.Write8(effAddr, val)

	case OpLDRH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint32(interp.Memory.Read16(effAddr))
		interp.CPU.SetReg(inst.RegDst, val)

	case OpSTRH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint16(getRegValue(inst.RegDst) & 0xFFFF)
		interp.Memory.Write16(effAddr, val)

	case OpLDRSB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val8 := int8(interp.Memory.Read8(effAddr))
		interp.CPU.SetReg(inst.RegDst, uint32(int32(val8)))

	case OpLDRSH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val16 := int16(interp.Memory.Read16(effAddr))
		interp.CPU.SetReg(inst.RegDst, uint32(int32(val16)))

	case OpPUSH:
		regList := uint16(inst.Imm)
		sp := interp.CPU.GetReg(SP)
		for r := 15; r >= 0; r-- {
			if (regList & (1 << uint(r))) != 0 {
				val := interp.CPU.GetReg(uint8(r))
				interp.Memory.Push32(&sp, val)
			}
		}
		interp.CPU.SetReg(SP, sp)

	case OpPOP:
		regList := uint16(inst.Imm)
		sp := interp.CPU.GetReg(SP)
		for r := 0; r <= 15; r++ {
			if (regList & (1 << uint(r))) != 0 {
				val := interp.Memory.Pop32(&sp)
				interp.CPU.SetReg(uint8(r), val)
			}
		}
		interp.CPU.SetReg(SP, sp)

	case OpB:
		interp.CPU.SetPC(inst.Imm)

	case OpBL:
		interp.CPU.SetReg(LR, nextPC)
		interp.CPU.SetPC(inst.Imm)

	case OpBX:
		target := interp.CPU.GetReg(inst.RegSrc1)
		interp.CPU.SetPC(target)

	case OpBLX:
		interp.CPU.SetReg(LR, nextPC)
		target := interp.CPU.GetReg(inst.RegSrc1)
		interp.CPU.SetPC(target)

	case OpSVC:
		if err := interp.HandleSyscall(); err != nil {
			return err
		}

	case OpHALT:
		interp.CPU.Halted = true

	case OpESCAPE:
		rawInst := inst.Imm
		if err := interp.HostBridge.Execute(interp.CPU, interp.Memory, rawInst, pc); err != nil {
			return fmt.Errorf("native host fallback error at PC 0x%08X (raw 0x%08X): %w", pc, rawInst, err)
		}

	default:
		return fmt.Errorf("unsupported opcode 0x%02X (%s) at PC 0x%08X", uint8(inst.Op), inst.Op.String(), pc)
	}

	return nil
}

func (interp *Interpreter) stepARMv8(inst Instruction, pc, nextPC uint32) error {
	// In ARMv8 ALU, X31 is XZR (Zero Register, reading returns 0)
	getReg64 := func(r uint8) uint64 {
		if r == 31 {
			return 0
		}
		return interp.CPU.GetReg64(r)
	}

	// In ARMv8 ALU, writes to X31 (XZR) are discarded
	setReg64 := func(r uint8, val uint64) {
		if r != 31 {
			interp.CPU.SetReg64(r, val)
		}
	}

	// In memory operations, register 31 is SP (Stack Pointer)
	getBaseAddr := func(r uint8) uint32 {
		return uint32(interp.CPU.GetReg64(r))
	}

	getOperand2_64 := func() uint64 {
		if inst.HasImm {
			return uint64(inst.Imm)
		}
		return getReg64(inst.RegSrc2)
	}

	switch inst.Op {
	case OpNOP:
		// No operation

	case OpMOV:
		op2 := getOperand2_64()
		if inst.Is64Bit {
			setReg64(inst.RegDst, op2)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(op2)
			}
		} else {
			val32 := uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, val32)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(val32)
			}
		}

	case OpMVN:
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := ^op2
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := ^uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpADD:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			var res uint64
			if inst.SetFlags {
				res = interp.CPU.UpdateAddFlags64(op1, op2, 0)
			} else {
				res = op1 + op2
			}
			setReg64(inst.RegDst, res)
		} else {
			var res uint32
			if inst.SetFlags {
				res = interp.CPU.UpdateAddFlags(uint32(op1), uint32(op2), 0)
			} else {
				res = uint32(op1) + uint32(op2)
			}
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
		}

	case OpADC:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		carry := uint64(0)
		if interp.CPU.FlagC {
			carry = 1
		}
		if inst.Is64Bit {
			var res uint64
			if inst.SetFlags {
				res = interp.CPU.UpdateAddFlags64(op1, op2, carry)
			} else {
				res = op1 + op2 + carry
			}
			setReg64(inst.RegDst, res)
		} else {
			var res uint32
			if inst.SetFlags {
				res = interp.CPU.UpdateAddFlags(uint32(op1), uint32(op2), uint32(carry))
			} else {
				res = uint32(op1) + uint32(op2) + uint32(carry)
			}
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
		}

	case OpSUB:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			var res uint64
			if inst.SetFlags {
				res = interp.CPU.UpdateSubFlags64(op1, op2, 0)
			} else {
				res = op1 - op2
			}
			setReg64(inst.RegDst, res)
		} else {
			var res uint32
			if inst.SetFlags {
				res = interp.CPU.UpdateSubFlags(uint32(op1), uint32(op2), 0)
			} else {
				res = uint32(op1) - uint32(op2)
			}
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
		}

	case OpSBC:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		borrow := uint64(1)
		if interp.CPU.FlagC {
			borrow = 0
		}
		if inst.Is64Bit {
			var res uint64
			if inst.SetFlags {
				res = interp.CPU.UpdateSubFlags64(op1, op2, borrow)
			} else {
				res = op1 - op2 - borrow
			}
			setReg64(inst.RegDst, res)
		} else {
			var res uint32
			if inst.SetFlags {
				res = interp.CPU.UpdateSubFlags(uint32(op1), uint32(op2), uint32(borrow))
			} else {
				res = uint32(op1) - uint32(op2) - uint32(borrow)
			}
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
		}

	case OpMUL:
		op1 := getReg64(inst.RegSrc1)
		op2 := getReg64(inst.RegSrc2)
		if inst.Is64Bit {
			res := op1 * op2
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := uint32(op1) * uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpAND:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := op1 & op2
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := uint32(op1) & uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpORR:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := op1 | op2
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := uint32(op1) | uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpEOR:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := op1 ^ op2
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := uint32(op1) ^ uint32(op2)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpBIC:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := op1 & (^op2)
			setReg64(inst.RegDst, res)
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ64(res)
			}
		} else {
			res := uint32(op1) & (^uint32(op2))
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, res)
			}
			if inst.SetFlags {
				interp.CPU.UpdateFlagsNZ(res)
			}
		}

	case OpCMP:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			interp.CPU.UpdateSubFlags64(op1, op2, 0)
		} else {
			interp.CPU.UpdateSubFlags(uint32(op1), uint32(op2), 0)
		}

	case OpCMN:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			interp.CPU.UpdateAddFlags64(op1, op2, 0)
		} else {
			interp.CPU.UpdateAddFlags(uint32(op1), uint32(op2), 0)
		}

	case OpTST:
		op1 := getReg64(inst.RegSrc1)
		op2 := getOperand2_64()
		if inst.Is64Bit {
			res := op1 & op2
			interp.CPU.UpdateFlagsNZ64(res)
		} else {
			res := uint32(op1) & uint32(op2)
			interp.CPU.UpdateFlagsNZ(res)
		}

	case OpLDRL:
		effAddr := inst.Imm
		if inst.Is64Bit {
			val := interp.Memory.Read64(effAddr)
			setReg64(inst.RegDst, val)
		} else {
			val := interp.Memory.Read32(effAddr)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, val)
			}
		}

	case OpLDR:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		if inst.Is64Bit {
			val := interp.Memory.Read64(effAddr)
			setReg64(inst.RegDst, val)
		} else {
			val := interp.Memory.Read32(effAddr)
			if inst.RegDst != 31 {
				interp.CPU.SetReg(inst.RegDst, val)
			}
		}

	case OpLDRSW:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		val := int64(int32(interp.Memory.Read32(effAddr)))
		setReg64(inst.RegDst, uint64(val))

	case OpSTR:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		if inst.Is64Bit {
			val := interp.CPU.GetReg64(inst.RegDst)
			interp.Memory.Write64(effAddr, val)
		} else {
			val := uint32(interp.CPU.GetReg64(inst.RegDst))
			interp.Memory.Write32(effAddr, val)
		}

	case OpLDRB:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		val := uint32(interp.Memory.Read8(effAddr))
		if inst.RegDst != 31 {
			interp.CPU.SetReg(inst.RegDst, val)
		}

	case OpSTRB:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		val := uint8(interp.CPU.GetReg64(inst.RegDst) & 0xFF)
		interp.Memory.Write8(effAddr, val)

	case OpLDRH:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		val := uint32(interp.Memory.Read16(effAddr))
		if inst.RegDst != 31 {
			interp.CPU.SetReg(inst.RegDst, val)
		}

	case OpSTRH:
		base := getBaseAddr(inst.RegSrc1)
		effAddr := base + inst.Imm
		val := uint16(interp.CPU.GetReg64(inst.RegDst) & 0xFFFF)
		interp.Memory.Write16(effAddr, val)

	case OpSTP:
		base := getBaseAddr(inst.RegSrc1)
		offset := int64(int32(inst.Imm))
		addr := uint32(int64(base) + offset)
		if inst.ShiftType == 1 { // post-indexed access at base
			addr = base
		}
		val1 := interp.CPU.GetReg64(inst.RegDst)
		val2 := interp.CPU.GetReg64(inst.RegSrc2)
		if inst.Is64Bit {
			interp.Memory.Write64(addr, val1)
			interp.Memory.Write64(addr+8, val2)
		} else {
			interp.Memory.Write32(addr, uint32(val1))
			interp.Memory.Write32(addr+4, uint32(val2))
		}
		if inst.SetFlags { // writeback requested (pre-index or post-index)
			setReg64(inst.RegSrc1, uint64(int64(base)+offset))
		}

	case OpLDP:
		base := getBaseAddr(inst.RegSrc1)
		offset := int64(int32(inst.Imm))
		addr := uint32(int64(base) + offset)
		if inst.ShiftType == 1 { // post-indexed access at base
			addr = base
		}
		if inst.Is64Bit {
			val1 := interp.Memory.Read64(addr)
			val2 := interp.Memory.Read64(addr + 8)
			setReg64(inst.RegDst, val1)
			setReg64(inst.RegSrc2, val2)
		} else {
			val1 := interp.Memory.Read32(addr)
			val2 := interp.Memory.Read32(addr + 4)
			setReg64(inst.RegDst, uint64(val1))
			setReg64(inst.RegSrc2, uint64(val2))
		}
		if inst.SetFlags { // writeback requested (pre-index or post-index)
			setReg64(inst.RegSrc1, uint64(int64(base)+offset))
		}

	case OpB:
		interp.CPU.SetPC(inst.Imm)

	case OpBL:
		interp.CPU.SetReg64(30, uint64(nextPC)) // X30 is Link Register
		interp.CPU.SetPC(inst.Imm)

	case OpBX: // RET Xn or BR Xn
		target := uint32(interp.CPU.GetReg64(inst.RegSrc1))
		if target == 0 {
			interp.CPU.Halted = true
			return nil
		}
		interp.CPU.SetPC(target)

	case OpBLX: // BLR Xn
		interp.CPU.SetReg64(30, uint64(nextPC))
		target := uint32(interp.CPU.GetReg64(inst.RegSrc1))
		if target == 0 {
			interp.CPU.Halted = true
			return nil
		}
		interp.CPU.SetPC(target)

	case OpSVC:
		return interp.HandleSyscall()

	case OpHALT:
		interp.CPU.Halted = true

	case OpESCAPE:
		rawInst := inst.Imm
		if err := interp.HostBridge.Execute(interp.CPU, interp.Memory, rawInst, pc); err != nil {
			return fmt.Errorf("native host fallback error at PC 0x%08X (raw 0x%08X): %w", pc, rawInst, err)
		}

	default:
		return fmt.Errorf("unsupported opcode 0x%02X (%s) at PC 0x%08X", uint8(inst.Op), inst.Op.String(), pc)
	}

	return nil
}

/* Run executes instructions until VM halts or encounters an error */
func (interp *Interpreter) Exec() (int, error) {
	for !interp.CPU.Halted {
		if err := interp.Step(); err != nil {
			return 1, err
		}
	}
	return interp.CPU.ExitCode, nil
}

/* Run is an alias for Exec */
func (interp *Interpreter) Run() (int, error) {
	return interp.Exec()
}
