package vm

import (
	"os"
	"testing"
	"time"
)

func TestHostSyscallBridgeGetppidARMv8(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv8
	interp.CPU.SetReg64(8, 173) // arm8_getppid

	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}

	expectedPPID := uint64(os.Getppid())
	actualPPID := interp.CPU.GetReg64(0)
	if actualPPID != expectedPPID {
		t.Errorf("Expected X0 to be PPID %d, got %d", expectedPPID, actualPPID)
	}

	if interp.HostSyscallBridge.DirectSyscallCount != 1 {
		t.Errorf("Expected DirectSyscallCount to be 1, got %d", interp.HostSyscallBridge.DirectSyscallCount)
	}
}

func TestHostSyscallBridgeGetuidARMv7(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv7
	interp.CPU.SetReg(7, 24) // arm7_getuid

	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}

	expectedUID := uint32(os.Getuid())
	actualUID := interp.CPU.GetReg(0)
	if actualUID != expectedUID {
		t.Errorf("Expected R0 to be UID %d, got %d", expectedUID, actualUID)
	}

	if interp.HostSyscallBridge.DirectSyscallCount != 1 {
		t.Errorf("Expected DirectSyscallCount to be 1, got %d", interp.HostSyscallBridge.DirectSyscallCount)
	}
}

func TestHostSyscallBridgeGetcwdARMv8(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv8
	interp.CPU.SetReg64(8, 17)      // arm8_getcwd
	interp.CPU.SetReg64(0, 0x20000) // buffer address
	interp.CPU.SetReg64(1, 1024)    // size

	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}

	retAddr := interp.CPU.GetReg64(0)
	if retAddr != 0x20000 {
		t.Errorf("Expected X0 return buffer address 0x20000, got 0x%X", retAddr)
	}

	expectedWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}

	actualWd := interp.Memory.ReadString(0x20000)
	if actualWd != expectedWd {
		t.Errorf("Expected cwd '%s', got '%s'", expectedWd, actualWd)
	}
}

func TestHostSyscallBridgeTimeARMv7(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv7
	interp.CPU.SetReg(7, 13) // arm7_time

	nowBefore := time.Now().Unix()
	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}
	nowAfter := time.Now().Unix()

	gotTime := int64(interp.CPU.GetReg(0))
	if gotTime < nowBefore || gotTime > nowAfter {
		t.Errorf("Expected timestamp between %d and %d, got %d", nowBefore, nowAfter, gotTime)
	}
}

func TestHostSyscallBridgeCustomHandler(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv8

	interp.HostSyscallBridge.CustomHandler = func(interp *Interpreter, sysNum uint64, args []uint64) (uint64, bool, error) {
		if sysNum == 888 {
			return 123456, true, nil
		}
		return 0, false, nil
	}

	interp.CPU.SetReg64(8, 888)
	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}

	if interp.CPU.GetReg64(0) != 123456 {
		t.Errorf("Expected custom syscall return 123456, got %d", interp.CPU.GetReg64(0))
	}
}

func TestHostSyscallBridgeUnmappedENOSYS(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv8
	interp.CPU.SetReg64(8, 9999) // Unmapped syscall number

	err := interp.HandleSyscall()
	if err != nil {
		t.Fatalf("HandleSyscall failed: %v", err)
	}

	v := int64(-38)
	expected := uint64(v)
	actual := interp.CPU.GetReg64(0)
	if actual != expected {
		t.Errorf("Expected -ENOSYS (0x%X), got 0x%X", expected, actual)
	}
}

func TestInterpreterFullSyscallFallbackExecutionARMv8(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv8

	// Instructions:
	// 0x10000: MOVZ X8, #174 (sys_getuid)
	// 0x10004: SVC #0
	// 0x10008: MOVZ X8, #93 (sys_exit with X0 as uid)
	// 0x1000C: SVC #0
	instructions := []Instruction{
		{Op: OpMOV, Cond: CondAL, RegDst: 8, HasImm: true, Imm: 174, Is64Bit: true, Arch: ArchARMv8},
		{Op: OpSVC, Cond: CondAL, Arch: ArchARMv8},
		{Op: OpMOV, Cond: CondAL, RegDst: 8, HasImm: true, Imm: 93, Is64Bit: true, Arch: ArchARMv8},
		{Op: OpSVC, Cond: CondAL, Arch: ArchARMv8},
	}

	bytecode := EncodeSlice(instructions)
	ivmFile := &IVMFile{
		Header: Header{
			Arch:         ArchARMv8,
			CodeLoadAddr: 0x00010000,
			EntryPC:      0x00010000,
			CodeSize:     uint32(len(bytecode)),
		},
		Bytecode: bytecode,
	}

	err := interp.LoadIVMFile(ivmFile)
	if err != nil {
		t.Fatalf("LoadIVMFile failed: %v", err)
	}

	exitCode, err := interp.Exec()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	expectedExit := os.Getuid()
	if exitCode != expectedExit {
		t.Errorf("Expected exit code to be UID %d, got %d", expectedExit, exitCode)
	}
	if interp.HostSyscallBridge.DirectSyscallCount != 1 {
		t.Errorf("Expected DirectSyscallCount = 1, got %d", interp.HostSyscallBridge.DirectSyscallCount)
	}
}

func TestInterpreterFullSyscallFallbackExecutionARMv7(t *testing.T) {
	interp := NewInterpreter(false)
	interp.CPU.Arch = ArchARMv7

	// Instructions:
	// 0x10000: MOV R7, #24 (sys_getuid)
	// 0x10004: SVC #0
	// 0x10008: MOV R7, #1 (sys_exit with R0 as uid)
	// 0x1000C: SVC #0
	instructions := []Instruction{
		{Op: OpMOV, Cond: CondAL, RegDst: 7, HasImm: true, Imm: 24, Arch: ArchARMv7},
		{Op: OpSVC, Cond: CondAL, Arch: ArchARMv7},
		{Op: OpMOV, Cond: CondAL, RegDst: 7, HasImm: true, Imm: 1, Arch: ArchARMv7},
		{Op: OpSVC, Cond: CondAL, Arch: ArchARMv7},
	}

	bytecode := EncodeSlice(instructions)
	ivmFile := &IVMFile{
		Header: Header{
			Arch:         ArchARMv7,
			CodeLoadAddr: 0x00010000,
			EntryPC:      0x00010000,
			CodeSize:     uint32(len(bytecode)),
		},
		Bytecode: bytecode,
	}

	err := interp.LoadIVMFile(ivmFile)
	if err != nil {
		t.Fatalf("LoadIVMFile failed: %v", err)
	}

	exitCode, err := interp.Exec()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	expectedExit := os.Getuid()
	if exitCode != expectedExit {
		t.Errorf("Expected exit code to be UID %d, got %d", expectedExit, exitCode)
	}
	if interp.HostSyscallBridge.DirectSyscallCount != 1 {
		t.Errorf("Expected DirectSyscallCount = 1, got %d", interp.HostSyscallBridge.DirectSyscallCount)
	}
}
