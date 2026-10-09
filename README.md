# IVM — Impressive Virtual Machine for ARMv7

**IVM** is a custom 32-bit virtual machine system and bytecode format built entirely in **Go**. It takes compiled ARMv7 ELF binaries (or object files), disassembles and translates their machine instructions into a custom-designed **IVM ISA bytecode**, packs them into a compressed `.ivm` container binary, and executes them via a custom virtual machine interpreter.

---

## Features

- **Custom ISA Bytecode**: 35+ ARMv7 instructions mapped to a fixed 8-byte bytecode format (`.ivm`).
- **CLI Commands**:
  - `ivm --pack [--arch armv7] [-o out.ivm] <binary>` — Packs ARMv7 ELF/object files into `.ivm` containers.
  - `ivm --run [--debug] <binary.ivm>` — Interprets and executes `.ivm` VM containers.
  - `ivm --disasm <binary.ivm>` — Pretty-prints the packed IVM bytecode with disassembled custom assembly instructions.
  - Subcommand variants (`ivm pack ...`, `ivm run ...`, `ivm disasm ...`) are also supported.
- **ARMv7 Linux Syscall Emulation**: Full EABI syscall interception (`sys_write`, `sys_read`, `sys_exit`, `sys_brk`, `sys_open`, `sys_close`, `sys_getpid`, `sys_uname`, `sys_writev`).
- **Full Relocation Resolver**: Pure Go ELF parser (`debug/elf`) that resolves `R_ARM_ABS32`, `R_ARM_TARGET1`, `R_ARM_CALL`, and `R_ARM_JUMP24` relocations for object files (`.o`) and executables.
- **Zero External Dependencies**: 100% written in standard Go.

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
| `0x20` | `SVC` | ARMv7 Linux System Call |
| `0xFE` | `HALT` | Stop Virtual Machine execution |

---

## Building & Usage

### 1. Build the IVM CLI Tool
```bash
go build -o ivm ./cmd/ivm
```

### 2. Packing an ARMv7 Binary
Compile your ARMv7 assembly or C source into an ELF binary or object file:
```bash
clang --target=armv7-linux-gnueabi -c -o hello.o testdata/hello.s
```

Pack it into an `.ivm` binary container:
```bash
./ivm --pack testdata/hello.o -o hello.ivm
# Or using subcommands:
./ivm pack --arch armv7 testdata/hello.o -o hello.ivm
```

### 3. Running an IVM Container
Execute the `.ivm` file in the VM interpreter:
```bash
./ivm --run hello.ivm
# Or with subcommands and verbose debug register tracing:
./ivm run --debug hello.ivm
```

### 4. Disassembling an IVM File
Inspect the packed custom ISA instructions:
```bash
./ivm --disasm hello.ivm
```

---

## Testing

Run the full Go test suite:
```bash
go test ./...
```

Run test suite binaries (`add_numbers`, `hello`, `loop_sum`, `factorial`):
```bash
./ivm --pack testdata/add_numbers.o && ./ivm --run testdata/add_numbers.ivm
./ivm --pack testdata/hello.o && ./ivm --run testdata/hello.ivm
./ivm --pack testdata/loop_sum.o && ./ivm --run testdata/loop_sum.ivm
./ivm --pack testdata/factorial.o && ./ivm --run testdata/factorial.ivm
```
