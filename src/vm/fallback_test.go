package vm

import (
	"testing"
)

func TestHostFallbackCLZ(t *testing.T) {
	bridge := NewHostBridge(false)
	cpu := NewCPU()
	mem := NewMemory()

	// clz r0, r1 with r1 = 0x000000FF (24 leading zeros)
	rawCLZ := uint32(0xE16F0F11)
	cpu.SetReg(1, 0x000000FF)

	err := bridge.Execute(cpu, mem, rawCLZ, 0x1000)
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	if cpu.GetReg(0) != 24 {
		t.Errorf("Expected R0 = 24, got %d", cpu.GetReg(0))
	}
}

func TestHostFallbackREV(t *testing.T) {
	bridge := NewHostBridge(false)
	cpu := NewCPU()
	mem := NewMemory()

	// rev r0, r1 with r1 = 0x12345678 -> 0x78563412
	rawREV := uint32(0xE6BF0F11)
	cpu.SetReg(1, 0x12345678)

	err := bridge.Execute(cpu, mem, rawREV, 0x1000)
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	if cpu.GetReg(0) != 0x78563412 {
		t.Errorf("Expected R0 = 0x78563412, got 0x%08X", cpu.GetReg(0))
	}
}

func TestHostFallbackUMULL(t *testing.T) {
	bridge := NewHostBridge(false)
	cpu := NewCPU()
	mem := NewMemory()

	// umull r0, r1, r2, r3 (r0=RdLo, r1=RdHi, r2=Rs, r3=Rm)
	// r2 = 0x80000000, r3 = 4 -> 0x200000000 (RdHi=2, RdLo=0)
	rawUMULL := uint32(0xE0810392)
	cpu.SetReg(2, 0x80000000)
	cpu.SetReg(3, 4)

	err := bridge.Execute(cpu, mem, rawUMULL, 0x1000)
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	if cpu.GetReg(0) != 0x00000000 {
		t.Errorf("Expected RdLo = 0, got 0x%08X", cpu.GetReg(0))
	}
	if cpu.GetReg(1) != 0x00000002 {
		t.Errorf("Expected RdHi = 2, got 0x%08X", cpu.GetReg(1))
	}
}

func TestInterpreterEscapeDispatch(t *testing.T) {
	interp := NewInterpreter(false)
	// Put raw CLZ in InstructionMap as OpESCAPE
	rawCLZ := uint32(0xE16F0F11) // clz r0, r1
	interp.InstructionMap[0x10000] = Instruction{
		Op:   OpESCAPE,
		Cond: CondAL,
		Imm:  rawCLZ,
	}
	interp.InstructionMap[0x10004] = Instruction{
		Op:   OpHALT,
		Cond: CondAL,
	}

	interp.CPU.SetReg(PC, 0x10000)
	interp.CPU.SetReg(1, 0x000000FF) // 24 leading zeros

	exitCode, err := interp.Run()
	if err != nil {
		t.Fatalf("Interpreter run failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if interp.CPU.GetReg(0) != 24 {
		t.Errorf("Expected R0 = 24, got %d", interp.CPU.GetReg(0))
	}
	if interp.HostBridge.FallbackCount != 1 {
		t.Errorf("Expected FallbackCount = 1, got %d", interp.HostBridge.FallbackCount)
	}
}

func TestARMv8HostFallbackSDIV(t *testing.T) {
	bridge := NewHostBridge(false)
	cpu := NewCPU()
	cpu.Arch = ArchARMv8
	mem := NewMemory()

	cpu.SetReg64(0, 100)
	cpu.SetReg64(1, 4)

	// SDIV X0, X0, X1 (raw 0x9AC10C00)
	err := bridge.Execute(cpu, mem, 0x9AC10C00, 0x1000)
	if err != nil {
		t.Fatalf("ARMv8 SDIV fallback failed: %v", err)
	}

	if cpu.GetReg64(0) != 25 {
		t.Errorf("Expected X0 = 25, got %d", cpu.GetReg64(0))
	}
}
