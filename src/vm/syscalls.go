package vm

import (
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

/* HandleSyscall intercepts both ARMv7 (R7) and ARMv8 (X8) Linux syscalls */
func (interp *Interpreter) HandleSyscall() error {
	var sysNum uint32
	var rawSys uint64
	args := make([]uint64, 6)

	setResult := func(val uint64) {
		if interp.CPU.Arch == ArchARMv8 {
			interp.CPU.SetReg64(0, val)
		} else {
			interp.CPU.SetReg(0, uint32(val))
		}
	}

	if interp.CPU.Arch == ArchARMv8 { // ARMv8 Linux (AArch64)
		rawSys = interp.CPU.GetReg64(8) // Syscall number in X8
		for i := 0; i < 6; i++ {
			args[i] = interp.CPU.GetReg64(uint8(i))
		}

		switch rawSys {
		case 93, 94: // sys_exit, sys_exit_group on AArch64
			sysNum = SysExit
		case 64: // sys_write on AArch64
			sysNum = SysWrite
		case 63: // sys_read on AArch64
			sysNum = SysRead
		case 214: // sys_brk on AArch64
			sysNum = SysBrk
		case 56: // sys_openat on AArch64 (path in x1)
			sysNum = SysOpen
			args[0] = args[1]
		case 57: // sys_close on AArch64
			sysNum = SysClose
		case 160: // sys_uname on AArch64
			sysNum = SysUname
		case 172: // sys_getpid on AArch64
			sysNum = SysGetPID
		case 66: // sys_writev on AArch64
			sysNum = SysWriteV
		default:
			sysNum = 0 // Dispatch to HostSyscallBridge
		}
	} else { // ARMv7 Linux
		rawSys = uint64(interp.CPU.GetReg(7)) // Syscall number in R7
		for i := 0; i < 6; i++ {
			args[i] = uint64(interp.CPU.GetReg(uint8(i)))
		}

		switch uint32(rawSys) {
		case SysExit, SysExitGroup:
			sysNum = SysExit
		case SysWrite:
			sysNum = SysWrite
		case SysRead:
			sysNum = SysRead
		case SysOpen:
			sysNum = SysOpen
		case SysClose:
			sysNum = SysClose
		case SysGetPID:
			sysNum = SysGetPID
		case SysBrk:
			sysNum = SysBrk
		case SysUname:
			sysNum = SysUname
		case SysWriteV:
			sysNum = SysWriteV
		default:
			sysNum = 0 // Dispatch to HostSyscallBridge
		}
	}

	arg0 := uint32(args[0])
	arg1 := uint32(args[1])
	arg2 := uint32(args[2])

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
			written, err = syscall.Write(fd, bytes)
		}

		if err != nil {
			setResult(uint64(toUint32(-9))) // -EBADF
		} else {
			setResult(uint64(written))
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
			setResult(uint64(toUint32(-9))) // -EBADF
		} else {
			interp.Memory.WriteBytes(bufAddr, buf[:n])
			setResult(uint64(n))
		}
		return nil

	case SysBrk:
		reqAddr := arg0
		if reqAddr == 0 {
			setResult(uint64(interp.Memory.Break))
		} else {
			if reqAddr > interp.Memory.Break {
				interp.Memory.Break = reqAddr
			}
			setResult(uint64(interp.Memory.Break))
		}
		return nil

	case SysGetPID:
		pid := os.Getpid()
		setResult(uint64(pid))
		return nil

	case SysOpen:
		path := interp.Memory.ReadString(arg0)
		flags := int(arg1)
		mode := uint32(arg2)
		fd, err := syscall.Open(path, flags, mode)
		if err != nil {
			setResult(uint64(toUint32(-1)))
		} else {
			setResult(uint64(fd))
		}
		return nil

	case SysClose:
		fd := int(arg0)
		if fd > 2 {
			syscall.Close(fd)
		}
		setResult(0)
		return nil

	case SysWriteV:
		fd := int(arg0)
		iovAddr := arg1
		iovCnt := int(arg2)

		totalWritten := 0
		for i := range iovCnt {
			base := interp.Memory.Read32(iovAddr + uint32(i*8))
			len := interp.Memory.Read32(iovAddr + uint32(i*8+4))
			data := interp.Memory.ReadBytes(base, len)

			var n int
			switch fd {
			case 1:
				n, _ = os.Stdout.Write(data)
			case 2:
				n, _ = os.Stderr.Write(data)
			default:
				n, _ = syscall.Write(fd, data)
			}
			totalWritten += n
		}
		setResult(uint64(totalWritten))
		return nil

	case SysUname:
		bufAddr := arg0
		writeStringPadded := func(offset uint32, str string) {
			b := make([]byte, 65)
			copy(b, []byte(str))
			interp.Memory.WriteBytes(bufAddr+offset, b)
		}
		archName := "armv7l"
		if interp.CPU.Arch == ArchARMv8 {
			archName = "aarch64"
		}
		writeStringPadded(0, "Linux")
		writeStringPadded(65, "ivm-host")
		writeStringPadded(130, "5.15.0-ivm")
		writeStringPadded(195, "#1 IVM Virtual Machine")
		writeStringPadded(260, archName)
		setResult(0)
		return nil

	default:
		if interp.HostSyscallBridge == nil {
			interp.HostSyscallBridge = NewHostSyscallBridge(interp.Debug)
		}
		res, err := interp.HostSyscallBridge.Execute(interp, rawSys, args)
		if err != nil {
			return err
		}
		setResult(res)
		return nil
	}
}
