package vm

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

/* Linux ARMv7 Syscall Numbers */
const (
	SysExit      uint32 = 1
	SysRead      uint32 = 3
	SysWrite     uint32 = 4
	SysOpen      uint32 = 5
	SysClose     uint32 = 6
	SysGetPID    uint32 = 20
	SysBrk       uint32 = 45
	SysMunmap    uint32 = 91
	SysUname     uint32 = 122
	SysWriteV    uint32 = 146
	SysMmap2     uint32 = 192
	SysExitGroup uint32 = 252
)

func toUint32(val int32) uint32 {
	return uint32(val)
}

/* HandleSyscall intercepts ARMv7 Linux syscalls */
func (interp *Interpreter) HandleSyscall() error {
	sysNum := interp.CPU.GetReg(7)

	arg0 := interp.CPU.GetReg(0)
	arg1 := interp.CPU.GetReg(1)
	arg2 := interp.CPU.GetReg(2)
	arg3 := interp.CPU.GetReg(3)

	switch sysNum {

	case SysExit, SysExitGroup:
		interp.CPU.Halted = true
		interp.CPU.ExitCode = int(arg0)
		return nil

	case SysWrite:
		fd := int(arg0)
		bufAddr := arg1
		count := arg2

		bytes := interp.Memory.ReadBytes(bufAddr, count)

		var written int
		var err error

		switch fd {
		case 1:
			written, err = os.Stdout.Write(bytes)
		case 2:
			written, err = os.Stderr.Write(bytes)
		default:
			// Write to arbitrary file descriptor if open in host
			written, err = syscall.Write(fd, bytes)
		}

		if err != nil {
			interp.CPU.SetReg(0, toUint32(-9)) // -EBADF
		} else {
			interp.CPU.SetReg(0, uint32(written))
		}
		return nil

	case SysRead:
		fd := int(arg0)
		bufAddr := arg1
		count := arg2

		buf := make([]byte, count)
		var n int
		var err error

		if fd == 0 {
			n, err = os.Stdin.Read(buf)
		} else {
			n, err = syscall.Read(fd, buf)
		}

		if err != nil && err != io.EOF {
			interp.CPU.SetReg(0, toUint32(-9)) // -EBADF
		} else {
			interp.Memory.WriteBytes(bufAddr, buf[:n])
			interp.CPU.SetReg(0, uint32(n))
		}
		return nil

	case SysBrk:
		reqAddr := arg0
		if reqAddr == 0 {
			interp.CPU.SetReg(0, interp.Memory.Break)
		} else {
			if reqAddr > interp.Memory.Break {
				interp.Memory.Break = reqAddr
			}
			interp.CPU.SetReg(0, interp.Memory.Break)
		}
		return nil

	case SysGetPID:
		pid := os.Getpid()
		interp.CPU.SetReg(0, uint32(pid))
		return nil

	case SysOpen:
		path := interp.Memory.ReadString(arg0)
		flags := int(arg1)
		mode := uint32(arg2)
		fd, err := syscall.Open(path, flags, mode)
		if err != nil {
			interp.CPU.SetReg(0, toUint32(-1)) // -1
		} else {
			interp.CPU.SetReg(0, uint32(fd))
		}
		return nil

	case SysClose:
		fd := int(arg0)
		if fd > 2 {
			syscall.Close(fd)
		}
		interp.CPU.SetReg(0, 0)
		return nil

	case SysWriteV:
		fd := int(arg0)
		iovAddr := arg1
		iovCnt := int(arg2)

		totalWritten := 0
		for i := 0; i < iovCnt; i++ {
			base := interp.Memory.Read32(iovAddr + uint32(i*8))
			len := interp.Memory.Read32(iovAddr + uint32(i*8+4))
			data := interp.Memory.ReadBytes(base, len)

			var n int
			if fd == 1 {
				n, _ = os.Stdout.Write(data)
			} else if fd == 2 {
				n, _ = os.Stderr.Write(data)
			} else {
				n, _ = syscall.Write(fd, data)
			}
			totalWritten += n
		}
		interp.CPU.SetReg(0, uint32(totalWritten))
		return nil

	case SysUname:
		bufAddr := arg0
		// struct utsname (65 bytes per field on Linux: sysname, nodename, release, version, machine)
		writeStringPadded := func(offset uint32, str string) {
			b := make([]byte, 65)
			copy(b, []byte(str))
			interp.Memory.WriteBytes(bufAddr+offset, b)
		}
		writeStringPadded(0, "Linux")
		writeStringPadded(65, "ivm-host")
		writeStringPadded(130, "5.15.0-ivm")
		writeStringPadded(195, "#1 IVM Virtual Machine")
		writeStringPadded(260, "armv7l")
		interp.CPU.SetReg(0, 0)
		return nil

	default:
		// Unimplemented syscall fallback
		if interp.Debug {
			fmt.Printf("[IVM VM Syscall] Warning: Unhandled syscall #%d (args: 0x%X, 0x%X, 0x%X, 0x%X)\n", sysNum, arg0, arg1, arg2, arg3)
		}
		interp.CPU.SetReg(0, 0)
		return nil
	}
}
