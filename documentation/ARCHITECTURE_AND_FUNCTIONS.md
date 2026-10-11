# IVM — Architecture & Function-by-Function Reference

Welcome to the comprehensive reference guide for **IVM (Independent Virtual Machine)**. This document explains the system architecture and every individual function across all packages in clear, accessible detail.

---

## High-Level Architecture

IVM translates ARMv7 (32-bit) and ARMv8 (64-bit AArch64) machine instructions into a custom-designed, fixed 8-byte bytecode format and executes them within an isolated virtual CPU and memory environment.

1. **The Packer (`--pack`)**: Reads an ARMv7 or ARMv8 ELF binary/object file, resolves symbol relocations, translates ARM machine instructions into IVM ISA instructions, and outputs an `.ivm` container binary.
2. **The Interpreter (`--run`)**: Instantiates a virtual CPU supporting either 16 32-bit registers (R0-R15) or 32 64-bit registers (X0-X31 / SP) and condition flags, allocates dynamic 64KB page-table virtual memory, loads code and data segments, and interprets IVM bytecode step-by-step while intercepting Linux syscalls (R7 for ARMv7, X8 for ARMv8).
3. **The Host CPU Fallback Bridge (`OpESCAPE`)**: When an instruction outside the virtualized ISA is encountered, execution falls back directly to the host CPU (Apple Silicon ARM64), preserving register state and flags seamlessly.

---

## Project Structure

```
ivm/
├── cmd/ivm/main.go            # CLI interface parser and command handlers
├── src/
│   ├── architectures/         # Disassemblers, decoders, ELF parsers, and translators
│   │   ├── armv7/             # ARMv7 32-bit architecture implementation
│   │   └── armv8/             # ARMv8 64-bit AArch64 architecture implementation
│   ├── vm/                    # Custom ISA bytecode, container format (.ivm), CPU state, memory, interpreter, syscalls, host fallback
│   │   ├── isa.go             # ISA opcodes, registers, condition codes, instruction struct
│   │   ├── encoder.go         # Fixed 8-byte bytecode encoder
│   │   ├── decoder.go         # Fixed 8-byte bytecode decoder
│   │   ├── ivm_file.go        # .ivm container file reader and writer
│   │   ├── cpu.go             # Virtual CPU registers (32-bit/64-bit) and CPSR condition flags
│   │   ├── memory.go          # 64KB paged virtual memory manager
│   │   ├── syscalls.go        # Linux syscall emulation (ARMv7 R7 / ARMv8 X8 conventions)
│   │   ├── syscall_bridge.go  # Direct host syscall execution bridge for unhandled syscalls
│   │   ├── interpreter.go     # Architecture-aware instruction execution loops
│   │   └── fallback.go        # Native Host CPU Fallback execution bridge
│   └── disasm/                # Human-readable bytecode disassembler
├── resources/
│   ├── asmsource/             # Test assembly source files (.s)
│   └── bin/                   # Compiled ELF objects (.o) and packed IVM containers (.ivm)
└── documentation/             # Architectural documentation and guides
```

---

## 1. src/vm — Core Virtual Machine & Bytecode Engine

The `src/vm` package consolidates the custom bytecode specification, container file format, virtual CPU registers, page-table memory manager, interpreter loops, Linux syscall emulator, and host CPU fallback bridge.

### isa.go
- **`Opcode.String()`**
  - **Description**: Converts an opcode byte value (e.g. `0x03`) into its human-readable mnemonic string (`"ADD"`).
  - **Implementation**: Uses a `switch` statement over the `Opcode` byte enum to return the corresponding string representation.
- **`FormatReg(r uint8) string`**
  - **Description**: Formats an ARMv7 register index (0-15) as `r0`-`r12`, `sp`, `lr`, or `pc`.
- **`FormatReg8(r uint8, is64 bool) string`**
  - **Description**: Formats an ARMv8 register index (0-31) as 64-bit `x0`-`x30`, `sp`/`xzr`, or 32-bit `w0`-`w30`, `wsp`/`wzr`.

### encoder.go
- **`EncodeInstruction(inst Instruction) [8]byte`**
  - **Description**: Serializes an `Instruction` struct into a fixed 8-byte binary layout.
  - **Implementation**:
    1. `Byte 0`: Opcode byte.
    2. `Byte 1`: Packed bitfield containing `(Cond << 4) | (SetFlags << 3) | (HasImm << 2) | ShiftType`.
    3. `Bytes 2..3`: Packed registers:
       - For ARMv8: 16-bit packed layout: bits 0-4 `RegDst` (5b), bits 5-9 `RegSrc1` (5b), bits 10-14 `RegSrc2` (5b), bit 15 `Is64Bit` (1b).
       - For ARMv7: 4-bit standard layout: `(RegDst << 4) | RegSrc1` in byte 2, `(RegSrc2 << 4) | ShiftAmount` in byte 3.
    4. `Bytes 4..7`: 32-bit immediate or target address encoded in Little-Endian.
- **`EncodeSlice(instructions []Instruction) []byte`**
  - **Description**: Encodes a slice of `Instruction` structs into contiguous bytecode.

### decoder.go
- **`DecodeInstruction(buf [8]byte, arch ...uint16) Instruction`**
  - **Description**: Deserializes an 8-byte binary block into an `Instruction` struct based on target architecture (`ArchARMv7` or `ArchARMv8`).
- **`DecodeSlice(bytecode []byte, arch ...uint16) ([]Instruction, error)`**
  - **Description**: Converts a full byte buffer into a slice of decoded `Instruction` structs.

### ivm_file.go
- **`SaveToFile(filepath string) error`**
  - **Description**: Writes the `.ivm` binary container file to disk.
  - **Implementation**: Writes the header struct (`IVM\x01` magic bytes, architecture, load addresses, section sizes) in Little-Endian order, followed by Bytecode, RawCode image, and Data payloads.
- **`LoadFromFile(filepath string) (*IVMFile, error)`**
  - **Description**: Reads and validates an `.ivm` binary container file from disk.
  - **Implementation**: Validates magic bytes `IVM\x01` and supported architecture (`0x0001` ARMv7 or `0x0002` ARMv8), then extracts section payloads into memory.

### cpu.go
- **`NewCPU() *CPU`**
  - **Description**: Instantiates a fresh Virtual CPU state with 32 64-bit registers initialized to 0 and flags cleared.
- **`CPU.GetReg(r uint8) uint32`** / **`CPU.SetReg(r uint8, val uint32)`**
  - **Description**: 32-bit register access (clearing upper 32 bits on write, as per AArch64 specification).
- **`CPU.GetReg64(r uint8) uint64`** / **`CPU.SetReg64(r uint8, val uint64)`**
  - **Description**: Full 64-bit register access for ARMv8.
- **`CPU.GetPC() uint32`** / **`CPU.SetPC(val uint32)`**
  - **Description**: Unified Program Counter accessor maintaining synchronization with R15 for ARMv7 while preserving independent X15 for ARMv8.
- **`CPU.CheckCondition(cond uint8) bool`**
  - **Description**: Evaluates condition codes (`EQ`, `NE`, `CS`, `CC`, `MI`, `PL`, `VS`, `VC`, `HI`, `LS`, `GE`, `LT`, `GT`, `LE`, `AL`).
- **`CPU.UpdateFlagsNZ(res uint32)`** / **`CPU.UpdateFlagsNZ64(res uint64)`**
  - **Description**: Updates Zero (`Z`) and Negative (`N`) flags for 32-bit or 64-bit results.
- **`CPU.UpdateAddFlags(a, b, carryIn uint32) uint32`** / **`CPU.UpdateAddFlags64(a, b, carryIn uint64) uint64`**
  - **Description**: Updates `N`, `Z`, `C`, `V` flags for 32-bit and 64-bit additions.
- **`CPU.UpdateSubFlags(a, b, borrowIn uint32) uint32`** / **`CPU.UpdateSubFlags64(a, b, borrowIn uint64) uint64`**
  - **Description**: Updates `N`, `Z`, `C`, `V` flags for 32-bit and 64-bit subtractions.
- **`CPU.DumpState() string`**
  - **Description**: Formats CPU register state (R0-R15 for ARMv7, X0-X31/SP for ARMv8).

### memory.go
- **`Memory.Read64(addr uint32) uint64`** / **`Memory.Write64(addr uint32, val uint64)`**
  - **Description**: 64-bit Little-Endian memory read and write.
- **`Memory.Push64(sp *uint32, val uint64)`** / **`Memory.Pop64(sp *uint32) uint64`**
  - **Description**: 64-bit stack operations.
- Page-table backed memory supporting 64KB pages with lazy allocation.

### syscalls.go
- **`Interpreter.HandleSyscall() error`**
  - **Description**: Emulates Linux syscalls:
    - ARMv7: Syscall number in `R7`, arguments in `R0`–`R5`, return value in `R0`.
    - ARMv8: Syscall number in `X8` (93 exit, 64 write, 63 read, 214 brk, 56 openat, 57 close, 160 uname, 172 getpid, 66 writev), arguments in `X0`–`X5`, return value in `X0`.
    - If a syscall is unhandled internally by IVM, it delegates immediately to `HostSyscallBridge.Execute()`.

### syscall_bridge.go
- **`NewHostSyscallBridge(debug bool) *HostSyscallBridge`**
  - **Description**: Creates a new syscall bridge instance for direct host execution of unhandled syscalls.
- **`HostSyscallBridge.Execute(interp *Interpreter, rawSys uint64, args []uint64) (uint64, error)`**
  - **Description**: Executes unhandled system calls directly on the host OS:
    - Processes custom hooks if `CustomHandler` is registered.
    - Translates ARMv7 and ARMv8 syscalls (`getppid`, `getuid`, `geteuid`, `getgid`, `getegid`, `gettid`, `getcwd`, `chdir`, `lseek`, `unlink`/`unlinkat`, `mkdir`/`mkdirat`, `rmdir`, `dup`/`dup2`/`dup3`, `pipe`/`pipe2`, `fchmod`/`chmod`, `fsync`, `fstat`/`stat`/`newfstatat`, `time`, `gettimeofday`, `clock_gettime`, `nanosleep`) to host OS equivalents, performing memory buffer translations.
    - On Linux hosts, performs raw direct syscalls via `syscall.Syscall6`.
    - On non-Linux hosts (e.g. Darwin macOS), returns POSIX `-ENOSYS` (-38) for unmapped calls.

### interpreter.go
- **`Interpreter.LoadIVMFile(ivmFile *IVMFile) error`**
  - **Description**: Initializes virtual memory, decodes bytecode with architecture awareness, sets stack pointer (`SP = 0x7FFFFF00` / `X31`), and initializes `PC`.
- **`Interpreter.Step() error`**
  - **Description**: Executes a single instruction cycle, dispatching to `stepARMv7` or `stepARMv8`.
- **`Interpreter.stepARMv7(inst, pc, nextPC)`**
  - **Description**: ARMv7 32-bit instruction interpreter loop with PC+8 pipeline evaluation.
- **`Interpreter.stepARMv8(inst, pc, nextPC)`**
  - **Description**: ARMv8 64-bit instruction interpreter loop with `XZR` zero-register semantics, `X30` link register, and pair load/store (`STP`/`LDP`).
- **`Interpreter.Run() (int, error)`**
  - **Description**: Executes instructions until halt, returning the program exit code.

### fallback.go
- **`HostBridge.Execute(cpu *CPU, mem *Memory, rawInst uint32, currPC uint32) error`**
  - **Description**: Dispatches unvirtualized instructions (`OpESCAPE`) to host CPU execution. For ARMv8, handles hardware division (`SDIV`/`UDIV`), multiply-accumulate (`MADD`/`MSUB`), leading zero counts (`CLZ`), and byte reversals (`REV`).

---

## 2. src/architectures/armv7 — ARMv7 Disassembler, ELF Parser & Translator

Parses ARMv7 ELF binaries, resolves relocations, and translates 32-bit ARM instructions into IVM bytecode.

### arm_decoder.go
- **`DecodeARM32(raw uint32) (*DecodedARM, error)`**
  - **Description**: Parses a raw 32-bit ARM machine instruction into a `DecodedARM` representation covering arithmetic, logical, memory transfers, branches, and system calls.

### elf_parser.go
- **`ParseELF(filepath string) (*ELFBinary, error)`**
  - **Description**: Reads ARMv7 ELF binaries or object files (`.o`), verifying `EM_ARM` machine type, extracting sections, and resolving relocations (`R_ARM_ABS32`, `R_ARM_TARGET1`, `R_ARM_CALL`, `R_ARM_JUMP24`).

### translator.go
- **`TranslateELF(elfBin *ELFBinary) (*TranslationResult, error)`**
  - **Description**: Converts an ARMv7 `ELFBinary` into an `IVMFile` container with `vm.ArchARMv7`.

---

## 3. src/architectures/armv8 — ARMv8 (AArch64) Decoder, ELF Parser & Translator

Parses 64-bit ARMv8 AArch64 ELF binaries, resolves relocations, and translates instructions into IVM bytecode.

### armv8_decoder.go
- **`DecodeAArch64(raw uint32, currPC uint32) (*DecodedARM8, error)`**
  - **Description**: Decodes a 32-bit ARMv8 instruction into a `DecodedARM8` representation. Supports:
    - Moves: `MOVZ`, `MOVN`, `MOV` register (`ORR Xd, XZR, Xm`)
    - Arithmetic: `ADD`, `SUB`, `CMP`, `CMN`, `ADDS`, `SUBS` (immediate and register)
    - Multiply: `MUL`
    - Logical: `AND`, `ORR`, `EOR`, `BIC`, `TST`
    - Memory: `LDR`, `STR`, `LDRB`, `STRB`, `LDRH`, `STRH`
    - Pair Memory: `STP`, `LDP` (pre-indexed, post-indexed, signed offset)
    - Literal Load: `LDR` literal (PC-relative constant pool load)
    - Branches: `B`, `B.cond`, `BL`, `RET`, `BR`, `BLR`
    - System: `SVC`
    - Fallback: Unrecognized instructions decode to `OpESCAPE` with raw machine code in `Imm` for Host CPU Bridge execution.

### elf_parser.go
- **`ParseELF(filepath string) (*ELFBinary, error)`**
  - **Description**: Reads ARMv8 ELF binaries or object files (`.o`), verifying `EM_AARCH64` machine type, resolving entry points (`_start` or `main`), and applying AArch64 relocations (`R_AARCH64_CALL26`, `R_AARCH64_JUMP26`, `R_AARCH64_ABS64`, `R_AARCH64_ABS32`).

### translator.go
- **`TranslateELF(elfBin *ELFBinary) (*TranslationResult, error)`**
  - **Description**: Converts an ARMv8 `ELFBinary` into an `IVMFile` container with `vm.ArchARMv8`.

---

## 4. src/disasm — Bytecode Disassembler

### disasm.go
- **`DisassembleIVM(ivmFile *vm.IVMFile) (string, error)`**
  - **Description**: Disassembles packed IVM bytecode into human-readable assembly, displaying target architecture and formatting registers appropriately (`R` for ARMv7, `X`/`W` for ARMv8).
- **`FormatInstruction(inst vm.Instruction, currAddr uint32) string`**
  - **Description**: Formats an individual `Instruction` struct into assembly string.

---

## 5. cmd/ivm — CLI Application Entry Point

### main.go
- **`handlePack(inputFile, arch, outputFile string) error`**
  - **Description**: Routes packing to `armv7` or `armv8` based on `--arch` flag (`armv7`, `armv8`, `aarch64`, `arm64`).
- **`handleRun(ivmFilepath string, debug bool) error`**
  - **Description**: Loads `.ivm` container and runs interpreter with optional debug state tracing.
- **`handleDisasm(ivmFilepath string) error`**
  - **Description**: Pretty-prints container disassembly.
