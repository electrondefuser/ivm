package vm

import (
	"fmt"
	"ivm/src/format"
	"ivm/src/isa"
)

type Interpreter struct {
	CPU              *CPU
	Memory           *Memory
	InstructionMap   map[uint32]isa.Instruction
	Debug            bool
	InstructionCount uint64
}

func NewInterpreter(debug bool) *Interpreter {
	return &Interpreter{
		CPU:            NewCPU(),
		Memory:         NewMemory(),
		InstructionMap: make(map[uint32]isa.Instruction),
		Debug:          debug,
	}
}

/* EvaluateShift computes operand shifted by shiftType and shiftAmount */
func EvaluateShift(val uint32, shiftType uint8, amount uint8) uint32 {
	if amount == 0 {
		return val
	}
	amount &= 31
	switch shiftType {
	case isa.ShiftLSL:
		return val << amount
	case isa.ShiftLSR:
		return val >> amount
	case isa.ShiftASR:
		return uint32(int32(val) >> amount)
	case isa.ShiftROR:
		return (val >> amount) | (val << (32 - amount))
	default:
		return val
	}
}

/* LoadIVMFile initializes virtual memory, CPU registers, and instruction map from IVMFile */
func (interp *Interpreter) LoadIVMFile(ivmFile *format.IVMFile) error {
	instructions, err := isa.DecodeSlice(ivmFile.Bytecode)
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

	// Setup stack (default SP at 0x7FFFFF00)
	spAddr := uint32(0x7FFFFF00)
	interp.CPU.SetReg(isa.SP, spAddr)

	// Set PC to entry point
	interp.CPU.SetReg(isa.PC, ivmFile.Header.EntryPC)

	return nil
}

/* Step executes a single IVM instruction at CPU.PC */
func (interp *Interpreter) Step() error {
	if interp.CPU.Halted {
		return nil
	}

	pc := interp.CPU.GetReg(isa.PC)
	inst, exists := interp.InstructionMap[pc]

	if !exists {
		return fmt.Errorf("execution error: no instruction mapped at PC 0x%08X\n%s", pc, interp.CPU.DumpState())
	}

	// Advance PC by 4 to point to next instruction by default
	nextPC := pc + 4
	interp.CPU.SetReg(isa.PC, nextPC)
	interp.InstructionCount++

	// Check condition code
	if !interp.CPU.CheckCondition(inst.Cond) {
		return nil
	}

	// Helper to get register value with ARM PC (+8) pipeline offset
	getRegValue := func(r uint8) uint32 {
		if r == isa.PC {
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
	case isa.OpNOP:
		// No operation

	case isa.OpMOV:
		op2 := getOperand2()
		interp.CPU.SetReg(inst.RegDst, op2)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(op2)
		}

	case isa.OpMVN:
		op2 := getOperand2()
		res := ^op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpADD:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateAddFlags(op1, op2, 0)
		} else {
			res = op1 + op2
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case isa.OpADC:
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

	case isa.OpSUB:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateSubFlags(op1, op2, 0)
		} else {
			res = op1 - op2
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case isa.OpSBC:
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

	case isa.OpRSB:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		var res uint32
		if inst.SetFlags {
			res = interp.CPU.UpdateSubFlags(op2, op1, 0)
		} else {
			res = op2 - op1
		}
		interp.CPU.SetReg(inst.RegDst, res)

	case isa.OpMUL:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getRegValue(inst.RegSrc2)
		res := op1 * op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpMLA:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getRegValue(inst.RegSrc2)
		acc := interp.CPU.GetReg(inst.RegDst)
		res := (op1 * op2) + acc
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpAND:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpORR:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 | op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpEOR:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 ^ op2
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpBIC:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & (^op2)
		interp.CPU.SetReg(inst.RegDst, res)
		if inst.SetFlags {
			interp.CPU.UpdateFlagsNZ(res)
		}

	case isa.OpCMP:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		interp.CPU.UpdateSubFlags(op1, op2, 0)

	case isa.OpCMN:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		interp.CPU.UpdateAddFlags(op1, op2, 0)

	case isa.OpTST:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 & op2
		interp.CPU.UpdateFlagsNZ(res)

	case isa.OpTEQ:
		op1 := getRegValue(inst.RegSrc1)
		op2 := getOperand2()
		res := op1 ^ op2
		interp.CPU.UpdateFlagsNZ(res)

	case isa.OpLDR:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := interp.Memory.Read32(effAddr)
		interp.CPU.SetReg(inst.RegDst, val)

	case isa.OpSTR:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := getRegValue(inst.RegDst)
		interp.Memory.Write32(effAddr, val)

	case isa.OpLDRB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint32(interp.Memory.Read8(effAddr))
		interp.CPU.SetReg(inst.RegDst, val)

	case isa.OpSTRB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint8(getRegValue(inst.RegDst) & 0xFF)
		interp.Memory.Write8(effAddr, val)

	case isa.OpLDRH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint32(interp.Memory.Read16(effAddr))
		interp.CPU.SetReg(inst.RegDst, val)

	case isa.OpSTRH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val := uint16(getRegValue(inst.RegDst) & 0xFFFF)
		interp.Memory.Write16(effAddr, val)

	case isa.OpLDRSB:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val8 := int8(interp.Memory.Read8(effAddr))
		interp.CPU.SetReg(inst.RegDst, uint32(int32(val8)))

	case isa.OpLDRSH:
		base := getRegValue(inst.RegSrc1)
		offset := getOperand2()
		effAddr := base + offset
		val16 := int16(interp.Memory.Read16(effAddr))
		interp.CPU.SetReg(inst.RegDst, uint32(int32(val16)))

	case isa.OpPUSH:
		regList := uint16(inst.Imm)
		sp := interp.CPU.GetReg(isa.SP)
		for r := 15; r >= 0; r-- {
			if (regList & (1 << uint(r))) != 0 {
				val := interp.CPU.GetReg(uint8(r))
				interp.Memory.Push32(&sp, val)
			}
		}
		interp.CPU.SetReg(isa.SP, sp)

	case isa.OpPOP:
		regList := uint16(inst.Imm)
		sp := interp.CPU.GetReg(isa.SP)
		for r := 0; r <= 15; r++ {
			if (regList & (1 << uint(r))) != 0 {
				val := interp.Memory.Pop32(&sp)
				interp.CPU.SetReg(uint8(r), val)
			}
		}
		interp.CPU.SetReg(isa.SP, sp)

	case isa.OpB:
		interp.CPU.SetReg(isa.PC, inst.Imm)

	case isa.OpBL:
		interp.CPU.SetReg(isa.LR, nextPC)
		interp.CPU.SetReg(isa.PC, inst.Imm)

	case isa.OpBX:
		target := interp.CPU.GetReg(inst.RegSrc1)
		interp.CPU.SetReg(isa.PC, target)

	case isa.OpBLX:
		interp.CPU.SetReg(isa.LR, nextPC)
		target := interp.CPU.GetReg(inst.RegSrc1)
		interp.CPU.SetReg(isa.PC, target)

	case isa.OpSVC:
		if err := interp.HandleSyscall(); err != nil {
			return err
		}

	case isa.OpHALT:
		interp.CPU.Halted = true

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
