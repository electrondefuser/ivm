package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ivm/src/armv7"
	"ivm/src/disasm"
	"ivm/src/format"
	"ivm/src/vm"
)

const Banner = `
  ___  _   _  __  __
 |_ _|| | | ||  \/  |
  | | | | | || |\/| |   Impressive Virtual Machine
  | | \ \_/ /| |  | |   v1.0.0
 |___| \___/ |_|  |_|
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Helper to extract flags regardless of order
	args := os.Args[1:]

	// Mode 1: Subcommand format (pack, run, disasm, help)
	cmd := args[0]
	if cmd == "pack" {
		packCmd := flag.NewFlagSet("pack", flag.ExitOnError)
		arch := packCmd.String("arch", "armv7", "Target architecture")
		out := packCmd.String("o", "", "Output filename")
		packCmd.Parse(args[1:])
		if packCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Usage: ivm pack [--arch armv7] [-o out.ivm] <binary>\n")
			os.Exit(1)
		}
		if err := handlePack(packCmd.Arg(0), *arch, *out); err != nil {
			fmt.Fprintf(os.Stderr, "Error packing binary: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if cmd == "run" {
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		debug := runCmd.Bool("debug", false, "Enable verbose debugging")
		runCmd.Parse(args[1:])
		if runCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Usage: ivm run [--debug] <binary.ivm>\n")
			os.Exit(1)
		}
		if err := handleRun(runCmd.Arg(0), *debug); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing IVM file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if cmd == "disasm" {
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: ivm disasm <binary.ivm>\n")
			os.Exit(1)
		}
		if err := handleDisasm(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error disassembling file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Mode 2: Flag-based format (--pack, --run, --disasm)
	var packInput string
	var runInput string
	var disasmInput string
	var arch string = "armv7"
	var out string
	var debug bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--pack" || arg == "-pack" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				packInput = args[i+1]
				i++
			}
		} else if arg == "--run" || arg == "-run" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				runInput = args[i+1]
				i++
			}
		} else if arg == "--disasm" || arg == "-disasm" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				disasmInput = args[i+1]
				i++
			}
		} else if arg == "--arch" || arg == "-arch" {
			if i+1 < len(args) {
				arch = args[i+1]
				i++
			}
		} else if arg == "-o" || arg == "--out" {
			if i+1 < len(args) {
				out = args[i+1]
				i++
			}
		} else if arg == "--debug" || arg == "-debug" || arg == "-v" {
			debug = true
		} else if packInput == "" && runInput == "" && disasmInput == "" && !strings.HasPrefix(arg, "-") {
			// Positional file fallback
			if strings.HasSuffix(arg, ".ivm") {
				runInput = arg
			} else {
				packInput = arg
			}
		}
	}

	if packInput != "" {
		if err := handlePack(packInput, arch, out); err != nil {
			fmt.Fprintf(os.Stderr, "Error packing binary: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if runInput != "" {
		if err := handleRun(runInput, debug); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing IVM file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if disasmInput != "" {
		if err := handleDisasm(disasmInput); err != nil {
			fmt.Fprintf(os.Stderr, "Error disassembling file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	printUsage()
}

func handlePack(inputFile, arch, outputFile string) error {
	if strings.ToLower(arch) != "armv7" {
		return fmt.Errorf("unsupported architecture '%s' (only armv7 is currently supported)", arch)
	}

	if outputFile == "" {
		ext := filepath.Ext(inputFile)
		if ext != "" {
			outputFile = strings.TrimSuffix(inputFile, ext) + ".ivm"
		} else {
			outputFile = inputFile + ".ivm"
		}
	}

	fmt.Printf("[IVM Packer] Reading ARMv7 ELF binary: %s\n", inputFile)
	elfBin, err := armv7.ParseELF(inputFile)
	if err != nil {
		return err
	}

	fmt.Printf("[IVM Packer] Code Section: 0x%08X (Size: %d bytes)\n", elfBin.CodeAddr, len(elfBin.CodeBytes))
	fmt.Printf("[IVM Packer] Translating ARMv7 instructions to Custom IVM ISA...\n")

	res, err := armv7.TranslateELF(elfBin)
	if err != nil {
		return err
	}

	fmt.Printf("[IVM Packer] Translated %d ARMv7 instructions into %d IVM bytecode instructions.\n",
		res.Stats.ARMInstructionCount, res.Stats.IVMInstructionCount)

	if err := res.IVMFile.SaveToFile(outputFile); err != nil {
		return err
	}

	fmt.Printf("[IVM Packer] Successfully packed into IVM container: %s\n", outputFile)
	return nil
}

func handleRun(ivmFilepath string, debug bool) error {
	ivmFile, err := format.LoadFromFile(ivmFilepath)
	if err != nil {
		return err
	}

	if debug {
		fmt.Printf("[IVM Interpreter] Loaded IVM file: %s\n", ivmFilepath)
		fmt.Printf("[IVM Interpreter] Code load addr: 0x%08X, Entry PC: 0x%08X\n", ivmFile.Header.CodeLoadAddr, ivmFile.Header.EntryPC)
	}

	interp := vm.NewInterpreter(debug)
	if err := interp.LoadIVMFile(ivmFile); err != nil {
		return err
	}

	exitCode, err := interp.Exec()
	if err != nil {
		return err
	}

	if debug {
		fmt.Printf("\n[IVM Interpreter] Execution finished after %d instructions.\n", interp.InstructionCount)
		fmt.Printf("[IVM Interpreter] Final CPU State:\n%s\n", interp.CPU.DumpState())
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

func handleDisasm(ivmFilepath string) error {
	ivmFile, err := format.LoadFromFile(ivmFilepath)
	if err != nil {
		return err
	}

	disStr, err := disasm.DisassembleIVM(ivmFile)
	if err != nil {
		return err
	}

	fmt.Print(disStr)
	return nil
}

func printUsage() {
	fmt.Print(Banner)
	fmt.Println("Usage:")
	fmt.Println("  ivm --pack [--arch armv7] [-o out.ivm] <binary>   Pack ARMv7 ELF binary into .ivm file")
	fmt.Println("  ivm --run [--debug] <binary.ivm>                 Execute .ivm VM binary")
	fmt.Println("  ivm --disasm <binary.ivm>                        Disassemble .ivm file")
	fmt.Println("\nSubcommands:")
	fmt.Println("  ivm pack [--arch armv7] [-o out.ivm] <binary>")
	fmt.Println("  ivm run [--debug] <binary.ivm>")
	fmt.Println("  ivm disasm <binary.ivm>")
}
