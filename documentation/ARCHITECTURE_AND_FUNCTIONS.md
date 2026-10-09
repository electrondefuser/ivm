# IVM — Architecture & Function-by-Function Reference

Welcome to the comprehensive reference guide for **IVM (Independent Virtual Machine)**. This document explains the system architecture and every individual function across all packages in clear, accessible detail.

---

## High-Level Architecture

IVM translates ARMv7 machine instructions into a custom-designed, fixed 8-byte bytecode format and executes them within an isolated 32-bit virtual CPU and memory environment.

1. **The Packer (`--pack`)**: Reads an ARMv7 ELF binary or object file, resolves symbol relocations, translates ARM instructions into IVM ISA instructions, and outputs a `.ivm` container binary.
2. **The Interpreter (`--run`)**: Instantiates a virtual CPU with 16 registers (R0–R15) and CPSR flags, allocates 64KB page-table-backed virtual memory, loads code and data segments, and interprets IVM bytecode instructions step-by-step while intercepting ARMv7 Linux syscalls.

---

## Project Structure

```
ivm/
├── cmd/ivm/main.go            # CLI interface parser and command handlers
├── pkg/
│   ├── isa/                   # Custom ISA specification, opcode table, encoder, decoder
│   ├── format/                # .ivm container file reader and writer
│   ├── armv7/                 # ARM 32-bit disassembler, ELF parser, and translator
│   ├── vm/                    # Virtual CPU state, page-table memory, interpreter, syscalls
│   └── disasm/                # Human-readable bytecode disassembler
```

---

## 1. pkg/isa — Instruction Set Architecture Rules

Defines custom bytecode structures, registers, condition codes, and binary packing functions.

### isa.go
- **`Opcode.String()`**
  - **Description**: Converts an opcode byte value (e.g. `0x03`) into its human-readable mnemonic string (`"ADD"`).
  - **Implementation**: Uses a `switch` statement over the `Opcode` byte enum to return the corresponding string representation.

### encoder.go
- **`EncodeInstruction(inst Instruction) [8]byte`**
  - **Description**: Serializes a single `Instruction` struct into a fixed 8-byte binary layout.
  - **Implementation**:
    1. `Byte 0`: Opcode byte.
    2. `Byte 1`: Packed bitfield containing `(Cond << 4) | (SetFlags << 3) | (HasImm << 2) | ShiftType`.
    3. `Byte 2`: Packed registers `(RegDst << 4) | RegSrc1`.
    4. `Byte 3`: Packed registers `(RegSrc2 << 4) | ShiftAmount`.
    5. `Bytes 4..7`: 32-bit immediate or target address encoded in Little-Endian (`binary.LittleEndian.PutUint32`).
- **`EncodeSlice(instructions []Instruction) []byte`**
  - **Description**: Encodes a slice of `Instruction` structs into a single contiguous bytecode slice (`[]byte`).
  - **Implementation**: Iterates over instructions, calls `EncodeInstruction` for each, and appends the 8-byte binary blocks.
- **`FormatReg(r uint8) string`**
  - **Description**: Maps register index numbers (0–15) to standard ARM register names (`r0`–`r12`, `sp`, `lr`, `pc`).
  - **Implementation**: Returns `"sp"` for 13, `"lr"` for 14, `"pc"` for 15, and `rN` for general-purpose registers.

### decoder.go
- **`DecodeInstruction(buf [8]byte) Instruction`**
  - **Description**: Deserializes an 8-byte binary block into an `Instruction` struct.
  - **Implementation**: Extracts bitfields using bitmask (`&`) and shift (`>>`) operations for condition codes, flag flags, registers, and reads the 32-bit immediate value via `binary.LittleEndian.Uint32`.
- **`DecodeSlice(bytecode []byte) ([]Instruction, error)`**
  - **Description**: Converts a full byte buffer into a slice of decoded `Instruction` structs.
  - **Implementation**: Validates that buffer length is a multiple of 8, then decodes each 8-byte block sequentially.

---

## 2. pkg/format — Container File Format (.ivm)

Handles serialization and deserialization of `.ivm` container files on disk.

### ivm_file.go
- **`SaveToFile(filepath string) error`**
  - **Description**: Writes the `.ivm` binary container file to disk.
  - **Implementation**: Writes the header struct (`IVM\x01` magic bytes, load addresses, section sizes) in Little-Endian order, followed by the raw Bytecode, RawCode image, and Data payloads.
- **`LoadFromFile(filepath string) (*IVMFile, error)`**
  - **Description**: Reads and validates an `.ivm` binary container file from disk.
  - **Implementation**: Reads header bytes, validates magic bytes `IVM\x01` and architecture `0x0001` (ARMv7), then extracts section payloads into memory.

---

## 3. pkg/armv7 — ARM Disassembler, ELF Parser & Translator

Parses ARMv7 ELF binaries, resolves relocations, and translates 32-bit ARM instructions into IVM bytecode.

### arm_decoder.go
- **`rotateRight(val uint32, count uint32) uint32`**
  - **Description**: Performs a 32-bit bitwise right rotation.
- **`signExtend24(val uint32) int32`**
  - **Description**: Sign-extends a 24-bit signed integer into a 32-bit signed integer (`int32`).
- **`DecodeARM32(raw uint32) (*DecodedARM, error)`**
  - **Description**: Parses a raw 32-bit ARM machine instruction into a `DecodedARM` representation.
  - **Implementation**: Matches opcode bit patterns for data processing instructions, single memory transfers (`LDR`/`STR`), block data transfers (`PUSH`/`POP`/`LDM`/`STM`), branch instructions (`B`/`BL`/`BX`/`BLX`), and system calls (`SVC`).

### elf_parser.go
- **`ParseELF(filepath string) (*ELFBinary, error)`**
  - **Description**: Reads ARMv7 ELF binaries or object files (`.o`), extracting `.text`, `.data`, `.rodata`, and `.bss` sections.
  - **Implementation**: Uses `debug/elf`, verifies `EM_ARM` machine type, extracts code and data buffers, and triggers `applyRelocations` if parsing an unlinked object file (`ET_REL`).
- **`applyRelocations(elfFile *elf.File, codeSec *elf.Section, bin *ELFBinary, _ []elf.Symbol)`**
  - **Description**: Resolves symbol relocations for relocatable object files (`.o`).
  - **Implementation**: Parses the raw `SHT_SYMTAB` symbol table and processes `SHT_REL` relocation entries (`R_ARM_ABS32`, `R_ARM_TARGET1`, `R_ARM_CALL`, `R_ARM_JUMP24`), updating memory addresses and branch offset fields directly in raw code bytes.

### translator.go
- **`TranslateELF(elfBin *ELFBinary) (*TranslationResult, error)`**
  - **Description**: Converts an entire `ELFBinary` structure into an `IVMFile` container.
  - **Implementation**: Iterates through 4-byte ARM instructions, decodes them via `DecodeARM32`, translates them with `convertDecodedARMToIVM`, and packs the encoded bytecode.
- **`convertDecodedARMToIVM(dec *DecodedARM, currPC uint32) isa.Instruction`**
  - **Description**: Translates a single `DecodedARM` instruction into an IVM `Instruction` struct.
  - **Implementation**: Maps register indices, condition codes, flags, and calculates absolute target virtual addresses for branch instructions (`B` and `BL`).

---

## 4. pkg/vm — Virtual Machine Execution Engine

Provides the Virtual CPU, page-table memory manager, interpreter loop, and Linux syscall emulator.

### cpu.go
- **`NewCPU() *CPU`**
  - **Description**: Instantiates a fresh Virtual CPU state with registers initialized to 0 and flags cleared.
- **`CPU.GetReg(r uint8) uint32`**
  - **Description**: Returns the 32-bit unsigned value stored in register `r` (0–15).
- **`CPU.SetReg(r uint8, val uint32)`**
  - **Description**: Writes a 32-bit unsigned value into register `r` (0–15).
- **`CPU.CheckCondition(cond uint8) bool`**
  - **Description**: Evaluates an ARM condition code (`EQ`, `NE`, `CS`, `CC`, `MI`, `PL`, `VS`, `VC`, `HI`, `LS`, `GE`, `LT`, `GT`, `LE`, `AL`) against current CPSR flags.
- **`CPU.UpdateFlagsNZ(res uint32)`**
  - **Description**: Updates Zero (`Z`) and Negative (`N`) flags based on a 32-bit operation result.
- **`CPU.UpdateAddFlags(a, b, carryIn uint32) uint32`**
  - **Description**: Performs addition (`a + b + carryIn`) and updates `N`, `Z`, `C` (carry), and `V` (signed overflow) flags.
- **`CPU.UpdateSubFlags(a, b, borrowIn uint32) uint32`**
  - **Description**: Performs subtraction (`a - b - borrowIn`) and updates `N`, `Z`, `C` (no-borrow), and `V` flags according to ARM specification.
- **`CPU.DumpState() string`**
  - **Description**: Generates a formatted multiline debug summary of all 16 registers and CPSR flags.

### memory.go
- **`NewMemory() *Memory`**
  - **Description**: Creates a virtual memory manager using dynamic 64KB page tables.
- **`Memory.getPage(pageIdx uint16, create bool) []byte`**
  - **Description**: Retrieves or lazily allocates a 64KB page buffer for page index `pageIdx`.
- **`Memory.Read8(addr)` / `Write8(addr, val)`**
  - **Description**: Reads or writes a 8-bit byte at virtual address `addr`.
- **`Memory.Read16(addr)` / `Write16(addr, val)`**
  - **Description**: Reads or writes a 16-bit halfword (Little-Endian) at virtual address `addr`.
- **`Memory.Read32(addr)` / `Write32(addr, val)`**
  - **Description**: Reads or writes a 32-bit word (Little-Endian) at virtual address `addr`.
- **`Memory.ReadBytes(addr, len)` / `WriteBytes(addr, data)`**
  - **Description**: Reads or writes a slice of raw bytes.
- **`Memory.ReadString(addr uint32) string`**
  - **Description**: Reads a null-terminated string from memory starting at virtual address `addr`.
- **`Memory.Push32(sp *uint32, val uint32)`**
  - **Description**: Pushes a 32-bit word onto the stack (decrements `*sp` by 4 and writes value).
- **`Memory.Pop32(sp *uint32) uint32`**
  - **Description**: Pops a 32-bit word from the stack (reads value and increments `*sp` by 4).
- **`Memory.LoadSegment(startAddr uint32, data []byte)`**
  - **Description**: Copies binary payload data into virtual memory starting at `startAddr`.

### syscalls.go
- **`toUint32(val int32) uint32`**
  - **Description**: Reinterprets a signed 32-bit integer as an unsigned 32-bit integer.
- **`Interpreter.HandleSyscall() error`**
  - **Description**: Emulates ARMv7 Linux syscalls based on `R7` register value (`SysExit`, `SysWrite`, `SysRead`, `SysBrk`, `SysGetPID`, `SysOpen`, `SysClose`, `SysUname`, `SysExitGroup`).

### interpreter.go
- **`EvaluateShift(val uint32, shiftType uint8, amount uint8) uint32`**
  - **Description**: Applies ARM register shifts (`LSL`, `LSR`, `ASR`, `ROR`).
- **`NewInterpreter(debug bool) *Interpreter`**
  - **Description**: Instantiates a VM Interpreter engine with CPU and Memory components.
- **`Interpreter.LoadIVMFile(ivmFile *format.IVMFile) error`**
  - **Description**: Initializes virtual memory, builds instruction PC mapping, sets stack pointer `SP = 0x7FFFFF00`, and sets entry point `PC = EntryPC`.
- **`Interpreter.Step() error`**
  - **Description**: Executes a single IVM instruction cycle at `CPU.PC` with ARM PC+8 pipeline evaluation.
- **`Interpreter.Run() (int, error)`**
  - **Description**: Continuously invokes `Step()` until `CPU.Halted` is set to `true`. Returns the final exit status code.

---

## 5. pkg/disasm — Bytecode Disassembler

### disasm.go
- **`DisassembleIVM(ivmFile *format.IVMFile) (string, error)`**
  - **Description**: Generates human-readable disassembly text from an `IVMFile` container.
- **`FormatInstruction(inst isa.Instruction, currAddr uint32) string`**
  - **Description**: Formats an `Instruction` struct into standard ARM assembly syntax.
- **`formatCond(cond uint8) string`**
  - **Description**: Returns condition mnemonic suffixes (`EQ`, `NE`, `CS`, `CC`, `GE`, `LT`, etc.).

---

## 6. cmd/ivm — CLI Application Entry Point

### main.go
- **`main()`**
  - **Description**: CLI entry point supporting both flag-based options (`--pack`, `--run`, `--disasm`) and subcommands (`pack`, `run`, `disasm`).
- **`handlePack(inputFile, arch, outputFile string) error`**
  - **Description**: Executes the binary packing workflow (parsing ELF, translating code, writing `.ivm`).
- **`handleRun(ivmFilepath string, debug bool) error`**
  - **Description**: Executes the VM interpreter workflow (loading `.ivm`, running interpreter loop).
- **`handleDisasm(ivmFilepath string) error`**
  - **Description**: Executes the disassembly workflow.
- **`printUsage()`**
  - **Description**: Prints CLI usage information and available flags.
