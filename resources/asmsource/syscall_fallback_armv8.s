.global _start
.text
_start:
    // Call getppid (sys_getppid = 173 in AArch64)
    // Unhandled by IVM core; dispatched to HostSyscallBridge!
    mov x8, #173
    svc #0

    // Compare with 0: PPID is always positive
    cmp x0, #0
    b.gt ppid_ok

    // Failed
    mov x0, #1
    mov x8, #93
    svc #0

ppid_ok:
    // Success: exit 0
    mov x0, #0
    mov x8, #93
    svc #0
