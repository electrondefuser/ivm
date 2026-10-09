package disasm

import (
	"fmt"
	"strings"

	"ivm/src/format"
	"ivm/src/isa"
)

/* DisassembleIVM formats an IVMFile into human readable assembly output */
func DisassembleIVM(ivmFile *format.IVMFile) (string, error) {
	instructions, err := isa.DecodeSlice(ivmFile.Bytecode)
	if err != nil {
		return "", fmt.Errorf("failed to decode bytecode for disassembly: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "; IVM Binary Disassembly\n")
	fmt.Fprintf(&sb, "; Target Arch: ARMv7 (0x%04X)\n", ivmFile.Header.Arch)
	fmt.Fprintf(&sb, "; Code Load Addr: 0x%08X (Size: %d bytes, %d insts)\n", ivmFile.Header.CodeLoadAddr, ivmFile.Header.CodeSize, len(instructions))
	fmt.Fprintf(&sb, "; Data Load Addr: 0x%08X (Size: %d bytes)\n", ivmFile.Header.DataLoadAddr, ivmFile.Header.DataSize)
	fmt.Fprintf(&sb, "; Entry PC: 0x%08X\n\n", ivmFile.Header.EntryPC)

	codeBase := ivmFile.Header.CodeLoadAddr

	for i, inst := range instructions {
		currAddr := codeBase + uint32(i*4)
		encoded := isa.EncodeInstruction(inst)

		bytesHex := fmt.Sprintf("%02X %02X %02X %02X %02X %02X %02X %02X",
			encoded[0], encoded[1], encoded[2], encoded[3],
			encoded[4], encoded[5], encoded[6], encoded[7],
		)

		isEntry := (currAddr == ivmFile.Header.EntryPC)
		entryMark := "  "
		if isEntry {
			entryMark = "=>"
		}

		instStr := FormatInstruction(inst, currAddr)
		sb.WriteString(fmt.Sprintf("%s 0x%08X:  %-23s  %s\n", entryMark, currAddr, bytesHex, instStr))
	}

	return sb.String(), nil
}

func FormatInstruction(inst isa.Instruction, currAddr uint32) string {
	condSuffix := formatCond(inst.Cond)
	flagSuffix := ""

	if inst.SetFlags {
		flagSuffix = "S"
	}

	mnemonic := fmt.Sprintf("%s%s%s", inst.Op.String(), condSuffix, flagSuffix)

	dstReg := isa.FormatReg(inst.RegDst)
	src1Reg := isa.FormatReg(inst.RegSrc1)
	src2Reg := isa.FormatReg(inst.RegSrc2)

	getOp2Str := func() string {
		if inst.HasImm {
			return fmt.Sprintf("#0x%X", inst.Imm)
		}
		if inst.ShiftAmount > 0 {
			shiftNames := []string{"LSL", "LSR", "ASR", "ROR"}
			sName := shiftNames[inst.ShiftType&3]
			return fmt.Sprintf("%s, %s #%d", src2Reg, sName, inst.ShiftAmount)
		}
		return src2Reg
	}

	switch inst.Op {
	case isa.OpNOP:
		return mnemonic

	case isa.OpMOV, isa.OpMVN:
		return fmt.Sprintf("%-8s %s, %s", mnemonic, dstReg, getOp2Str())

	case isa.OpADD, isa.OpADC, isa.OpSUB, isa.OpSBC, isa.OpRSB, isa.OpAND, isa.OpORR, isa.OpEOR, isa.OpBIC:
		return fmt.Sprintf("%-8s %s, %s, %s", mnemonic, dstReg, src1Reg, getOp2Str())

	case isa.OpMUL:
		return fmt.Sprintf("%-8s %s, %s, %s", mnemonic, dstReg, src1Reg, src2Reg)

	case isa.OpMLA:
		return fmt.Sprintf("%-8s %s, %s, %s, %s", mnemonic, dstReg, src1Reg, src2Reg, isa.FormatReg(inst.RegDst))

	case isa.OpCMP, isa.OpCMN, isa.OpTST, isa.OpTEQ:
		return fmt.Sprintf("%-8s %s, %s", mnemonic, src1Reg, getOp2Str())

	case isa.OpLDR, isa.OpLDRB, isa.OpLDRH, isa.OpLDRSB, isa.OpLDRSH:
		return fmt.Sprintf("%-8s %s, [%s, %s]", mnemonic, dstReg, src1Reg, getOp2Str())

	case isa.OpSTR, isa.OpSTRB, isa.OpSTRH:
		return fmt.Sprintf("%-8s %s, [%s, %s]", mnemonic, dstReg, src1Reg, getOp2Str())

	case isa.OpPUSH:
		return fmt.Sprintf("%-8s {mask: 0x%04X}", mnemonic, inst.Imm)

	case isa.OpPOP:
		return fmt.Sprintf("%-8s {mask: 0x%04X}", mnemonic, inst.Imm)

	case isa.OpB, isa.OpBL:
		return fmt.Sprintf("%-8s 0x%08X", mnemonic, inst.Imm)

	case isa.OpBX, isa.OpBLX:
		return fmt.Sprintf("%-8s %s", mnemonic, src1Reg)

	case isa.OpSVC:
		return fmt.Sprintf("%-8s #0x%X", mnemonic, inst.Imm)

	case isa.OpHALT:
		return mnemonic

	default:
		return fmt.Sprintf("%-8s %s, %s, %s, #0x%X", mnemonic, dstReg, src1Reg, src2Reg, inst.Imm)
	}
}

func formatCond(cond uint8) string {
	switch cond {
	case isa.CondEQ:
		return "EQ"
	case isa.CondNE:
		return "NE"
	case isa.CondCS:
		return "CS"
	case isa.CondCC:
		return "CC"
	case isa.CondMI:
		return "MI"
	case isa.CondPL:
		return "PL"
	case isa.CondVS:
		return "VS"
	case isa.CondVC:
		return "VC"
	case isa.CondHI:
		return "HI"
	case isa.CondLS:
		return "LS"
	case isa.CondGE:
		return "GE"
	case isa.CondLT:
		return "LT"
	case isa.CondGT:
		return "GT"
	case isa.CondLE:
		return "LE"
	default:
		return ""
	}
}
