# IVM — Independent Virtual Machine for ARMv7 & ARMv8

**IVM** is a dual-architecture virtual machine system and custom bytecode format built entirely in **Go**. It takes compiled 32-bit ARMv7 or 64-bit ARMv8 (AArch64) ELF binaries (or object files), disassembles and translates their machine instructions into a custom-designed **IVM ISA bytecode**, packs them into an `.ivm` container binary, and executes them via a custom virtual machine interpreter.

---

## Features

- **Dual-Architecture Support**: Supports both ARMv7 (32-bit, `vm.ArchARMv7`) and ARMv8 (64-bit AArch64, `vm.ArchARMv8`).
- **Custom ISA Bytecode**: Fixed 8-byte bytecode layout supporting 32-bit registers (R0-R15) and 64-bit registers (X0-X31 / SP).
- **CLI Commands**:
  - `ivm --pack [--arch armv7|armv8] [-o out.ivm] <binary>` — Packs ARM ELF/object files into `.ivm` containers.
  - `ivm --run [--debug] <binary.ivm>` — Interprets and executes `.ivm` VM containers.
  - `ivm --disasm <binary.ivm>` — Pretty-prints the packed IVM bytecode with disassembled custom assembly instructions.
  - Subcommand variants (`ivm pack ...`, `ivm run ...`, `ivm disasm ...`) are also supported.
- **Linux Syscall Emulation & Direct Host Fallback**: Intercepts ARMv7 (`R7`) and ARMv8 (`X8`) Linux syscalls. If a syscall is not defined in IVM, the **Host Syscall Bridge** directly executes it on the host OS (`getppid`, `getuid`, `getcwd`, `lseek`, `fstat`, `time`, `clock_gettime`, `nanosleep`, etc.) or invokes direct host syscalls.
- **Host CPU Fallback Bridge**: Unrecognized or unvirtualized instructions automatically escape to physical host evaluation (`OpESCAPE`).
- **Full Relocation Resolver**: Pure Go ELF parser (`debug/elf`) that resolves relocations for both ARMv7 (`R_ARM_*`) and ARMv8 (`R_AARCH64_*`) object files.
- **Zero External Dependencies**: 100% written in standard Go without CGO or third-party packages.

---

## Custom IVM ISA Specification

IVM uses a **Fixed 8-Byte Instruction Layout**:

```
Byte 0: Opcode (8 bits)
Byte 1: [Cond: 4b][UpdateFlags: 1b][HasImm: 1b][ShiftType: 2b]
Byte 2: [RegDst: 4b][RegSrc1: 4b]
Byte 3: [RegSrc2: 4b][ShiftAmount: 4b]
Bytes 4..7: 32-bit Immediate / Offset / Memory Address (Little Endian uint32)
```

### Supported Opcodes

| Opcode | Name | Description |
|---|---|---|
| `0x00` | `NOP` | No operation |
| `0x01` | `MOV` | Move immediate or register value |
| `0x02` | `MVN` | Move bitwise NOT value |
| `0x03` | `ADD` | Add registers/immediates (`Rd = Rn + Rm/Imm`) |
| `0x04` | `ADC` | Add with Carry |
| `0x05` | `SUB` | Subtract (`Rd = Rn - Rm/Imm`) |
| `0x06` | `SBC` | Subtract with Carry |
| `0x07` | `RSB` | Reverse Subtract (`Rd = Rm/Imm - Rn`) |
| `0x08` | `MUL` | Multiply (`Rd = Rn * Rm`) |
| `0x09` | `MLA` | Multiply Accumulate |
| `0x0A` | `AND` | Bitwise AND |
| `0x0B` | `ORR` | Bitwise OR |
| `0x0C` | `EOR` | Bitwise XOR |
| `0x0D` | `BIC` | Bit Clear |
| `0x0E` | `CMP` | Compare (updates N, Z, C, V CPSR flags) |
| `0x0F` | `CMN` | Compare Negative |
| `0x10` | `TST` | Test bits |
| `0x11` | `TEQ` | Test Equivalence |
| `0x12` | `LDR` | Load 32-bit Word from Memory |
| `0x13` | `STR` | Store 32-bit Word to Memory |
| `0x14` | `LDRB` | Load Byte |
| `0x15` | `STRB` | Store Byte |
| `0x16` | `LDRH` | Load 16-bit Halfword |
| `0x17` | `STRH` | Store 16-bit Halfword |
| `0x18` | `LDRSB`| Load Signed Byte |
| `0x19` | `LDRSH`| Load Signed Halfword |
| `0x1A` | `PUSH` | Push Register List to Stack |
| `0x1B` | `POP` | Pop Register List from Stack |
| `0x1C` | `B` | Branch to Target Address |
| `0x1D` | `BL` | Branch with Link (subroutine call) |
| `0x1E` | `BX` | Branch & Exchange to Register Target |
| `0x1F` | `BLX` | Branch with Link & Exchange |
| `0x20` | `SVC` | Linux System Call (ARMv7 R7 / ARMv8 X8 convention) |
| `0x23` | `STP` | Store Pair of Registers (`[Xn, #offset]`) |
| `0x24` | `LDP` | Load Pair of Registers (`[Xn, #offset]`) |
| `0x25` | `LDRL`| Load Literal (PC-relative data / constant pool) |
| `0xFD` | `ESCAPE` | Hand over unvirtualized instruction to Host CPU |
| `0xFE` | `HALT` | Stop Virtual Machine execution |

---

## Building & Usage

### 1. Build the IVM CLI Tool
```bash
go build -o bin/ivm ./cmd/ivm
```

### 2. Packing & Running ARMv7 (32-bit) Binaries
Assemble with ARMv7 target:
```bash
clang --target=armv7-linux-gnueabi -c resources/asmsource/hello.s -o resources/bin/armbinary/hello.o
```

Pack and execute:
```bash
./bin/ivm --pack --arch armv7 resources/bin/armbinary/hello.o -o resources/bin/ivm/hello.ivm
./bin/ivm --run resources/bin/ivm/hello.ivm
```

### 3. Packing & Running ARMv8 (64-bit) Binaries
Assemble with AArch64 target:
```bash
clang --target=aarch64-linux-gnu -c resources/asmsource/hello_armv8.s -o resources/bin/armbinary/hello_armv8.o
```

Pack and execute:
```bash
./bin/ivm --pack --arch armv8 resources/bin/armbinary/hello_armv8.o -o resources/bin/ivm/hello_armv8.ivm
./bin/ivm --run resources/bin/ivm/hello_armv8.ivm
```

### 4. Disassembling an IVM File
Inspect the packed custom ISA instructions:
```bash
./bin/ivm --disasm resources/bin/ivm/hello_armv8.ivm
```

### 5. Running with Debug Tracing
Inspect register dumps and step counts:
```bash
./bin/ivm --run --debug resources/bin/ivm/loop_sum_armv8.ivm
```

---

## Testing

Run the full Go test suite:
```bash
go test -v ./...
```
