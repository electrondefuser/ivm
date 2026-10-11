package disasm

import (
	"fmt"
	"strings"

	"ivm/src/vm"
)

/* DisassembleIVM formats an IVMFile into human readable assembly output */
func DisassembleIVM(ivmFile *vm.IVMFile) (string, error) {
	instructions, err := vm.DecodeSlice(ivmFile.Bytecode, ivmFile.Header.Arch)
	if err != nil {
		return "", fmt.Errorf("failed to decode bytecode for disassembly: %w", err)
	}

	var sb strings.Builder
	archName := "ARMv7 (32-bit)"
	if ivmFile.Header.Arch == vm.ArchARMv8 {
		archName = "ARMv8 (64-bit)"
	}
	fmt.Fprintf(&sb, "; IVM Binary Disassembly\n")
	fmt.Fprintf(&sb, "; Target Arch: %s (0x%04X)\n", archName, ivmFile.Header.Arch)
	fmt.Fprintf(&sb, "; Code Load Addr: 0x%08X (Size: %d bytes, %d insts)\n", ivmFile.Header.CodeLoadAddr, ivmFile.Header.CodeSize, len(instructions))
	fmt.Fprintf(&sb, "; Data Load Addr: 0x%08X (Size: %d bytes)\n", ivmFile.Header.DataLoadAddr, ivmFile.Header.DataSize)
	fmt.Fprintf(&sb, "; Entry PC: 0x%08X\n\n", ivmFile.Header.EntryPC)

	codeBase := ivmFile.Header.CodeLoadAddr

	for i, inst := range instructions {
		currAddr := codeBase + uint32(i*4)
		encoded := vm.EncodeInstruction(inst)

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
		fmt.Fprintf(&sb, "%s 0x%08X:  %-23s  %s\n", entryMark, currAddr, bytesHex, instStr)
	}

	return sb.String(), nil
}

func FormatInstruction(inst vm.Instruction, currAddr uint32) string {
	condSuffix := formatCond(inst.Cond)
	flagSuffix := ""

	if inst.SetFlags {
		flagSuffix = "S"
	}

	mnemonic := fmt.Sprintf("%s%s%s", inst.Op.String(), condSuffix, flagSuffix)

	var dstReg, src1Reg, src2Reg string
	if inst.Arch == vm.ArchARMv8 || inst.Is64Bit || inst.RegDst > 15 || inst.RegSrc1 > 15 || inst.RegSrc2 > 15 {
		dstReg = vm.FormatReg8(inst.RegDst, inst.Is64Bit)
		src1Reg = vm.FormatReg8(inst.RegSrc1, inst.Is64Bit)
		src2Reg = vm.FormatReg8(inst.RegSrc2, inst.Is64Bit)
	} else {
		dstReg = vm.FormatReg(inst.RegDst)
		src1Reg = vm.FormatReg(inst.RegSrc1)
		src2Reg = vm.FormatReg(inst.RegSrc2)
	}

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
	case vm.OpNOP:
		return mnemonic

	case vm.OpMOV, vm.OpMVN:
		return fmt.Sprintf("%-8s %s, %s", mnemonic, dstReg, getOp2Str())

	case vm.OpADD, vm.OpADC, vm.OpSUB, vm.OpSBC, vm.OpRSB, vm.OpAND, vm.OpORR, vm.OpEOR, vm.OpBIC:
		return fmt.Sprintf("%-8s %s, %s, %s", mnemonic, dstReg, src1Reg, getOp2Str())

	case vm.OpMUL:
		return fmt.Sprintf("%-8s %s, %s, %s", mnemonic, dstReg, src1Reg, src2Reg)

	case vm.OpMLA:
		return fmt.Sprintf("%-8s %s, %s, %s, %s", mnemonic, dstReg, src1Reg, src2Reg, dstReg)

	case vm.OpCMP, vm.OpCMN, vm.OpTST, vm.OpTEQ:
		return fmt.Sprintf("%-8s %s, %s", mnemonic, src1Reg, getOp2Str())

	case vm.OpLDR, vm.OpLDRB, vm.OpLDRH, vm.OpLDRSB, vm.OpLDRSH:
		return fmt.Sprintf("%-8s %s, [%s, %s]", mnemonic, dstReg, src1Reg, getOp2Str())

	case vm.OpLDRL:
		return fmt.Sprintf("%-8s %s, =0x%08X", mnemonic, dstReg, inst.Imm)

	case vm.OpSTR, vm.OpSTRB, vm.OpSTRH:
		return fmt.Sprintf("%-8s %s, [%s, %s]", mnemonic, dstReg, src1Reg, getOp2Str())

	case vm.OpSTP, vm.OpLDP:
		return fmt.Sprintf("%-8s %s, %s, [%s, #%d]", mnemonic, dstReg, src2Reg, src1Reg, int32(inst.Imm))

	case vm.OpPUSH:
		return fmt.Sprintf("%-8s {mask: 0x%04X}", mnemonic, inst.Imm)

	case vm.OpPOP:
		return fmt.Sprintf("%-8s {mask: 0x%04X}", mnemonic, inst.Imm)

	case vm.OpB, vm.OpBL:
		return fmt.Sprintf("%-8s 0x%08X", mnemonic, inst.Imm)

	case vm.OpBX, vm.OpBLX:
		return fmt.Sprintf("%-8s %s", mnemonic, src1Reg)

	case vm.OpSVC:
		return fmt.Sprintf("%-8s #0x%X", mnemonic, inst.Imm)

	case vm.OpHALT:
		return mnemonic

	case vm.OpESCAPE:
		return fmt.Sprintf("%-8s [RAW: 0x%08X]", mnemonic, inst.Imm)

	default:
		return fmt.Sprintf("%-8s %s, %s, %s, #0x%X", mnemonic, dstReg, src1Reg, src2Reg, inst.Imm)
	}
}

func formatCond(cond uint8) string {
	switch cond {
	case vm.CondEQ:
		return "EQ"
	case vm.CondNE:
		return "NE"
	case vm.CondCS:
		return "CS"
	case vm.CondCC:
		return "CC"
	case vm.CondMI:
		return "MI"
	case vm.CondPL:
		return "PL"
	case vm.CondVS:
		return "VS"
	case vm.CondVC:
		return "VC"
	case vm.CondHI:
		return "HI"
	case vm.CondLS:
		return "LS"
	case vm.CondGE:
		return "GE"
	case vm.CondLT:
		return "LT"
	case vm.CondGT:
		return "GT"
	case vm.CondLE:
		return "LE"
	default:
		return ""
	}
}
