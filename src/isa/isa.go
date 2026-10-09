package isa

import "fmt"

/* Magic bytes for .ivm files ("IVM\x01") */
var IVMMagic = [4]byte{'I', 'V', 'M', 0x01}

/* IVM Custom Opcodes (1 Byte each) */
type Opcode uint8

/*
 * Instruction represents a single decoded/parsed IVM instruction.
 *	Fixed 8-Byte Layout when encoded:
 * 	Byte 0: Opcode
 *	Byte 1: [Cond: 4b][UpdateFlags: 1b][HasImm: 1b][ShiftType: 2b]
 *	Byte 2: [RegDst: 4b][RegSrc1: 4b]
 *	Byte 3: [RegSrc2: 4b][ShiftAmount: 4b]
 *	Byte 4..7: Immediate / Offset / Address (32-bit uint32)
 */
type Instruction struct {
	Op          Opcode
	Cond        uint8  /* 4 bits (0-15) */
	SetFlags    bool   /* S bit */
	HasImm      bool   /* True if Imm field is active instead of RegSrc2 */
	ShiftType   uint8  /* LSL, LSR, ASR, ROR */
	ShiftAmount uint8  /* 0-31 */
	RegDst      uint8  /* Target register (Rd) */
	RegSrc1     uint8  /* Source register 1 (Rn) */
	RegSrc2     uint8  /* Source register 2 (Rm) / RegMask for LDM/STM */
	Imm         uint32 /* 32-bit Immediate value / Offset / Branch target */
}

/* Register definitions matching ARMv7 */
const (
	R0  uint8 = 0
	R1  uint8 = 1
	R2  uint8 = 2
	R3  uint8 = 3
	R4  uint8 = 4
	R5  uint8 = 5
	R6  uint8 = 6
	R7  uint8 = 7
	R8  uint8 = 8
	R9  uint8 = 9
	R10 uint8 = 10
	R11 uint8 = 11
	R12 uint8 = 12
	SP  uint8 = 13 // R13
	LR  uint8 = 14 // R14
	PC  uint8 = 15 // R15
)

/* Condition Codes (ARMv7 standard) */
const (
	CondEQ uint8 = 0  // Equal (Z=1)
	CondNE uint8 = 1  // Not Equal (Z=0)
	CondCS uint8 = 2  // Carry Set / Higher or Same (C=1)
	CondCC uint8 = 3  // Carry Clear / Lower (C=0)
	CondMI uint8 = 4  // Minus / Negative (N=1)
	CondPL uint8 = 5  // Plus / Positive or Zero (N=0)
	CondVS uint8 = 6  // Overflow Set (V=1)
	CondVC uint8 = 7  // Overflow Clear (V=0)
	CondHI uint8 = 8  // Unsigned Higher (C=1 && Z=0)
	CondLS uint8 = 9  // Unsigned Lower or Same (C=0 || Z=1)
	CondGE uint8 = 10 // Signed Greater or Equal (N==V)
	CondLT uint8 = 11 // Signed Less Than (N!=V)
	CondGT uint8 = 12 // Signed Greater Than (Z=0 && N==V)
	CondLE uint8 = 13 // Signed Less Than or Equal (Z=1 || N!=V)
	CondAL uint8 = 14 // Always (Unconditional)
	CondNV uint8 = 15 // Never / Reserved
)

/* Shift Types */
const (
	ShiftLSL uint8 = 0 // Logical Shift Left
	ShiftLSR uint8 = 1 // Logical Shift Right
	ShiftASR uint8 = 2 // Arithmetic Shift Right
	ShiftROR uint8 = 3 // Rotate Right
)

const (
	OpNOP     Opcode = 0x00
	OpMOV     Opcode = 0x01
	OpMVN     Opcode = 0x02
	OpADD     Opcode = 0x03
	OpADC     Opcode = 0x04
	OpSUB     Opcode = 0x05
	OpSBC     Opcode = 0x06
	OpRSB     Opcode = 0x07
	OpMUL     Opcode = 0x08
	OpMLA     Opcode = 0x09
	OpAND     Opcode = 0x0A
	OpORR     Opcode = 0x0B
	OpEOR     Opcode = 0x0C
	OpBIC     Opcode = 0x0D
	OpCMP     Opcode = 0x0E
	OpCMN     Opcode = 0x0F
	OpTST     Opcode = 0x10
	OpTEQ     Opcode = 0x11
	OpLDR     Opcode = 0x12
	OpSTR     Opcode = 0x13
	OpLDRB    Opcode = 0x14
	OpSTRB    Opcode = 0x15
	OpLDRH    Opcode = 0x16
	OpSTRH    Opcode = 0x17
	OpLDRSB   Opcode = 0x18
	OpLDRSH   Opcode = 0x19
	OpPUSH    Opcode = 0x1A
	OpPOP     Opcode = 0x1B
	OpB       Opcode = 0x1C
	OpBL      Opcode = 0x1D
	OpBX      Opcode = 0x1E
	OpBLX     Opcode = 0x1F
	OpSVC     Opcode = 0x20
	OpLDM     Opcode = 0x21
	OpSTM     Opcode = 0x22
	OpHALT    Opcode = 0xFE
	OpILLEGAL Opcode = 0xFF
)

/* String returns human-readable name of opcode */
func (op Opcode) String() string {
	switch op {
	case OpNOP:
		return "NOP"
	case OpMOV:
		return "MOV"
	case OpMVN:
		return "MVN"
	case OpADD:
		return "ADD"
	case OpADC:
		return "ADC"
	case OpSUB:
		return "SUB"
	case OpSBC:
		return "SBC"
	case OpRSB:
		return "RSB"
	case OpMUL:
		return "MUL"
	case OpMLA:
		return "MLA"
	case OpAND:
		return "AND"
	case OpORR:
		return "ORR"
	case OpEOR:
		return "EOR"
	case OpBIC:
		return "BIC"
	case OpCMP:
		return "CMP"
	case OpCMN:
		return "CMN"
	case OpTST:
		return "TST"
	case OpTEQ:
		return "TEQ"
	case OpLDR:
		return "LDR"
	case OpSTR:
		return "STR"
	case OpLDRB:
		return "LDRB"
	case OpSTRB:
		return "STRB"
	case OpLDRH:
		return "LDRH"
	case OpSTRH:
		return "STRH"
	case OpLDRSB:
		return "LDRSB"
	case OpLDRSH:
		return "LDRSH"
	case OpPUSH:
		return "PUSH"
	case OpPOP:
		return "POP"
	case OpB:
		return "B"
	case OpBL:
		return "BL"
	case OpBX:
		return "BX"
	case OpBLX:
		return "BLX"
	case OpSVC:
		return "SVC"
	case OpLDM:
		return "LDM"
	case OpSTM:
		return "STM"
	case OpHALT:
		return "HALT"
	default:
		return fmt.Sprintf("UNK(0x%02X)", uint8(op))
	}
}
