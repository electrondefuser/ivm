package vm

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

/*
 * HostSyscallBridge manages direct host execution of system calls that are not
 * handled or emulated internally by IVM. When the virtual machine encounters an
 * unhandled SVC (syscall), this bridge maps and executes the syscall directly on
 * the host OS, translates VM pointers/buffers where necessary, and returns the result.
 */
type HostSyscallBridge struct {
	Debug              bool
	DirectSyscallCount uint64
	CustomHandler      func(interp *Interpreter, sysNum uint64, args []uint64) (uint64, bool, error)
}

func NewHostSyscallBridge(debug bool) *HostSyscallBridge {
	return &HostSyscallBridge{
		Debug: debug,
	}
}

// Canonical Linux syscall numbers for reference
const (
	// Linux ARMv7 numbers
	arm7_unlink        uint32 = 10
	arm7_chdir         uint32 = 12
	arm7_time          uint32 = 13
	arm7_chmod         uint32 = 15
	arm7_lseek         uint32 = 19
	arm7_getuid        uint32 = 24
	arm7_mkdir         uint32 = 39
	arm7_rmdir         uint32 = 40
	arm7_dup           uint32 = 41
	arm7_pipe          uint32 = 42
	arm7_getgid        uint32 = 47
	arm7_geteuid       uint32 = 49
	arm7_getegid       uint32 = 50
	arm7_dup2          uint32 = 63
	arm7_getppid       uint32 = 64
	arm7_gettimeofday  uint32 = 78
	arm7_fchmod        uint32 = 94
	arm7_stat          uint32 = 106
	arm7_fstat         uint32 = 108
	arm7_fsync         uint32 = 118
	arm7_nanosleep     uint32 = 162
	arm7_getcwd        uint32 = 183
	arm7_fstat64       uint32 = 197
	arm7_gettid        uint32 = 224
	arm7_clock_gettime uint32 = 265
	arm7_mkdirat       uint32 = 322
	arm7_fstatat64     uint32 = 327
	arm7_unlinkat      uint32 = 328
	arm7_pipe2         uint32 = 359

	// Linux ARMv8 (AArch64) numbers
	arm8_getcwd        uint64 = 17
	arm8_dup           uint64 = 23
	arm8_dup3          uint64 = 24
	arm8_mkdirat       uint64 = 34
	arm8_unlinkat      uint64 = 35
	arm8_chdir         uint64 = 49
	arm8_fchmodat      uint64 = 52
	arm8_pipe2         uint64 = 59
	arm8_lseek         uint64 = 62
	arm8_newfstatat    uint64 = 79
	arm8_fstat         uint64 = 80
	arm8_fsync         uint64 = 82
	arm8_nanosleep     uint64 = 101
	arm8_clock_gettime uint64 = 113
	arm8_gettimeofday  uint64 = 169
	arm8_getppid       uint64 = 173
	arm8_getuid        uint64 = 174
	arm8_geteuid       uint64 = 175
	arm8_getgid        uint64 = 176
	arm8_getegid       uint64 = 177
	arm8_gettid        uint64 = 178
)

// Execute dispatches a syscall directly to the host OS.
func (b *HostSyscallBridge) Execute(interp *Interpreter, rawSys uint64, args []uint64) (uint64, error) {
	b.DirectSyscallCount++

	if b.Debug {
		fmt.Printf("[Host Syscall Bridge] Dispatched Syscall #%d (Args: 0x%X, 0x%X, 0x%X, 0x%X, 0x%X, 0x%X) | Host: %s/%s\n",
			rawSys, args[0], args[1], args[2], args[3], args[4], args[5], runtime.GOOS, runtime.GOARCH)
	}

	// 1. Custom handler hook if registered
	if b.CustomHandler != nil {
		res, handled, err := b.CustomHandler(interp, rawSys, args)
		if err != nil {
			return 0, err
		}
		if handled {
			if b.Debug {
				fmt.Printf("[Host Syscall Bridge] Custom handler handled syscall #%d -> 0x%X\n", rawSys, res)
			}
			return res, nil
		}
	}

	var res uint64
	var handled bool

	if interp.CPU.Arch == ArchARMv8 {
		res, handled = b.executeARMv8(interp, rawSys, args)
	} else {
		res, handled = b.executeARMv7(interp, rawSys, args)
	}

	if handled {
		if b.Debug {
			fmt.Printf("[Host Syscall Bridge] Handled syscall #%d -> 0x%X\n", rawSys, res)
		}
		return res, nil
	}

	// 2. Fallback for unmapped syscall on Linux host: Attempt raw host syscall
	if runtime.GOOS == "linux" {
		r1, _, errno := syscall.Syscall6(
			uintptr(rawSys),
			uintptr(args[0]), uintptr(args[1]), uintptr(args[2]),
			uintptr(args[3]), uintptr(args[4]), uintptr(args[5]),
		)
		if errno != 0 {
			res = b.makeErrResult(interp, int(errno))
		} else {
			res = uint64(r1)
		}
		if b.Debug {
			fmt.Printf("[Host Syscall Bridge] Raw Linux syscall #%d -> 0x%X\n", rawSys, res)
		}
		return res, nil
	}

	// 3. Fallback for unmapped syscall on non-Linux host (e.g. Darwin macOS):
	// Return -ENOSYS (-38) so caller receives POSIX function-not-implemented code
	if b.Debug {
		fmt.Printf("[Host Syscall Bridge] Warning: Syscall #%d unmapped on host OS %s; returning -ENOSYS (-38)\n",
			rawSys, runtime.GOOS)
	}
	return b.makeErrResult(interp, 38), nil // -ENOSYS
}

func (b *HostSyscallBridge) executeARMv8(interp *Interpreter, sys uint64, args []uint64) (uint64, bool) {
	switch sys {
	case arm8_getppid:
		return uint64(os.Getppid()), true

	case arm8_getuid:
		return uint64(os.Getuid()), true

	case arm8_geteuid:
		return uint64(os.Geteuid()), true

	case arm8_getgid:
		return uint64(os.Getgid()), true

	case arm8_getegid:
		return uint64(os.Getegid()), true

	case arm8_gettid:
		return uint64(os.Getpid()), true

	case arm8_getcwd:
		bufAddr := uint32(args[0])
		size := uint32(args[1])
		wd, err := os.Getwd()
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		if size > 0 && uint32(len(wd)+1) > size {
			return b.makeErrResult(interp, 34), true // -ERANGE
		}
		interp.Memory.WriteString(bufAddr, wd)
		return uint64(bufAddr), true

	case arm8_chdir:
		path := interp.Memory.ReadString(uint32(args[0]))
		err := os.Chdir(path)
		return b.errToLinux(interp, err), true

	case arm8_lseek:
		ret, err := syscall.Seek(int(args[0]), int64(args[1]), int(args[2]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return uint64(ret), true

	case arm8_unlinkat:
		path := interp.Memory.ReadString(uint32(args[1]))
		err := syscall.Unlink(path)
		return b.errToLinux(interp, err), true

	case arm8_mkdirat:
		path := interp.Memory.ReadString(uint32(args[1]))
		mode := uint32(args[2])
		err := syscall.Mkdir(path, mode)
		return b.errToLinux(interp, err), true

	case arm8_dup:
		nfd, err := syscall.Dup(int(args[0]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return uint64(nfd), true

	case arm8_dup3:
		err := syscall.Dup2(int(args[0]), int(args[1]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return args[1], true

	case arm8_pipe2:
		p := [2]int{}
		err := syscall.Pipe(p[:])
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		interp.Memory.Write32(uint32(args[0]), uint32(p[0]))
		interp.Memory.Write32(uint32(args[0]+4), uint32(p[1]))
		return 0, true

	case arm8_fsync:
		err := syscall.Fsync(int(args[0]))
		return b.errToLinux(interp, err), true

	case arm8_fchmodat:
		path := interp.Memory.ReadString(uint32(args[1]))
		mode := uint32(args[2])
		err := syscall.Chmod(path, mode)
		return b.errToLinux(interp, err), true

	case arm8_gettimeofday:
		now := time.Now()
		sec := now.Unix()
		usec := now.Nanosecond() / 1000
		interp.Memory.Write64(uint32(args[0]), uint64(sec))
		interp.Memory.Write64(uint32(args[0]+8), uint64(usec))
		return 0, true

	case arm8_clock_gettime:
		now := time.Now()
		sec := now.Unix()
		nsec := now.Nanosecond()
		interp.Memory.Write64(uint32(args[1]), uint64(sec))
		interp.Memory.Write64(uint32(args[1]+8), uint64(nsec))
		return 0, true

	case arm8_nanosleep:
		sec := interp.Memory.Read64(uint32(args[0]))
		nsec := interp.Memory.Read64(uint32(args[0] + 8))
		d := time.Duration(sec)*time.Second + time.Duration(nsec)*time.Nanosecond
		time.Sleep(d)
		return 0, true

	case arm8_fstat:
		var st syscall.Stat_t
		err := syscall.Fstat(int(args[0]), &st)
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		b.writeStat64(interp, uint32(args[1]), &st)
		return 0, true

	case arm8_newfstatat:
		path := interp.Memory.ReadString(uint32(args[1]))
		var st syscall.Stat_t
		err := syscall.Stat(path, &st)
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		b.writeStat64(interp, uint32(args[2]), &st)
		return 0, true

	default:
		return 0, false
	}
}

func (b *HostSyscallBridge) executeARMv7(interp *Interpreter, sys uint64, args []uint64) (uint64, bool) {
	switch uint32(sys) {
	case arm7_getppid:
		return uint64(os.Getppid()), true

	case arm7_getuid:
		return uint64(os.Getuid()), true

	case arm7_geteuid:
		return uint64(os.Geteuid()), true

	case arm7_getgid:
		return uint64(os.Getgid()), true

	case arm7_getegid:
		return uint64(os.Getegid()), true

	case arm7_gettid:
		return uint64(os.Getpid()), true

	case arm7_time:
		return uint64(time.Now().Unix()), true

	case arm7_getcwd:
		bufAddr := uint32(args[0])
		size := uint32(args[1])
		wd, err := os.Getwd()
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		if size > 0 && uint32(len(wd)+1) > size {
			return b.makeErrResult(interp, 34), true // -ERANGE
		}
		interp.Memory.WriteString(bufAddr, wd)
		return uint64(bufAddr), true

	case arm7_chdir:
		path := interp.Memory.ReadString(uint32(args[0]))
		err := os.Chdir(path)
		return b.errToLinux(interp, err), true

	case arm7_lseek:
		ret, err := syscall.Seek(int(args[0]), int64(int32(args[1])), int(args[2]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return uint64(uint32(ret)), true

	case arm7_unlink, arm7_unlinkat:
		pathAddr := uint32(args[0])
		if uint32(sys) == arm7_unlinkat {
			pathAddr = uint32(args[1])
		}
		path := interp.Memory.ReadString(pathAddr)
		err := syscall.Unlink(path)
		return b.errToLinux(interp, err), true

	case arm7_mkdir, arm7_mkdirat:
		pathAddr := uint32(args[0])
		mode := uint32(args[1])
		if uint32(sys) == arm7_mkdirat {
			pathAddr = uint32(args[1])
			mode = uint32(args[2])
		}
		path := interp.Memory.ReadString(pathAddr)
		err := syscall.Mkdir(path, mode)
		return b.errToLinux(interp, err), true

	case arm7_rmdir:
		path := interp.Memory.ReadString(uint32(args[0]))
		err := syscall.Rmdir(path)
		return b.errToLinux(interp, err), true

	case arm7_dup:
		nfd, err := syscall.Dup(int(args[0]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return uint64(uint32(nfd)), true

	case arm7_dup2:
		err := syscall.Dup2(int(args[0]), int(args[1]))
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		return uint64(uint32(args[1])), true

	case arm7_pipe, arm7_pipe2:
		p := [2]int{}
		err := syscall.Pipe(p[:])
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		interp.Memory.Write32(uint32(args[0]), uint32(p[0]))
		interp.Memory.Write32(uint32(args[0]+4), uint32(p[1]))
		return 0, true

	case arm7_chmod:
		path := interp.Memory.ReadString(uint32(args[0]))
		mode := uint32(args[1])
		err := syscall.Chmod(path, mode)
		return b.errToLinux(interp, err), true

	case arm7_fchmod:
		err := syscall.Fchmod(int(args[0]), uint32(args[1]))
		return b.errToLinux(interp, err), true

	case arm7_fsync:
		err := syscall.Fsync(int(args[0]))
		return b.errToLinux(interp, err), true

	case arm7_gettimeofday:
		now := time.Now()
		sec := uint32(now.Unix())
		usec := uint32(now.Nanosecond() / 1000)
		interp.Memory.Write32(uint32(args[0]), sec)
		interp.Memory.Write32(uint32(args[0]+4), usec)
		return 0, true

	case arm7_clock_gettime:
		now := time.Now()
		sec := uint32(now.Unix())
		nsec := uint32(now.Nanosecond())
		interp.Memory.Write32(uint32(args[1]), sec)
		interp.Memory.Write32(uint32(args[1]+4), nsec)
		return 0, true

	case arm7_nanosleep:
		sec := interp.Memory.Read32(uint32(args[0]))
		nsec := interp.Memory.Read32(uint32(args[0] + 4))
		d := time.Duration(sec)*time.Second + time.Duration(nsec)*time.Nanosecond
		time.Sleep(d)
		return 0, true

	case arm7_stat:
		path := interp.Memory.ReadString(uint32(args[0]))
		var st syscall.Stat_t
		err := syscall.Stat(path, &st)
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		b.writeStat32(interp, uint32(args[1]), &st)
		return 0, true

	case arm7_fstat, arm7_fstat64:
		var st syscall.Stat_t
		err := syscall.Fstat(int(args[0]), &st)
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		b.writeStat32(interp, uint32(args[1]), &st)
		return 0, true

	case arm7_fstatat64:
		path := interp.Memory.ReadString(uint32(args[1]))
		var st syscall.Stat_t
		err := syscall.Stat(path, &st)
		if err != nil {
			return b.errToLinux(interp, err), true
		}
		b.writeStat32(interp, uint32(args[2]), &st)
		return 0, true

	default:
		return 0, false
	}
}

func (b *HostSyscallBridge) writeStat64(interp *Interpreter, bufAddr uint32, st *syscall.Stat_t) {
	interp.Memory.Write32(bufAddr+16, uint32(st.Mode))
	interp.Memory.Write64(bufAddr+48, uint64(st.Size))
}

func (b *HostSyscallBridge) writeStat32(interp *Interpreter, bufAddr uint32, st *syscall.Stat_t) {
	interp.Memory.Write32(bufAddr+16, uint32(st.Mode))
	interp.Memory.Write32(bufAddr+44, uint32(st.Size))
	interp.Memory.Write64(bufAddr+48, uint64(st.Size))
}

func (b *HostSyscallBridge) makeErrResult(interp *Interpreter, errno int) uint64 {
	if interp.CPU.Arch == ArchARMv8 {
		return uint64(int64(-errno))
	}
	return uint64(uint32(int32(-errno)))
}

func (b *HostSyscallBridge) errToLinux(interp *Interpreter, err error) uint64 {
	if err == nil {
		return 0
	}
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return b.makeErrResult(interp, int(errno))
	}
	return b.makeErrResult(interp, 1) // -EPERM
}
