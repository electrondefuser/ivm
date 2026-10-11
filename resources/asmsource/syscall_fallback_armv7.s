.syntax unified
.arm
.global _start
.text
_start:
    // Call getppid (sys_getppid = 64 in ARMv7 Linux)
    // Unhandled by IVM core; dispatched to HostSyscallBridge!
    mov r7, #64
    svc #0

    // Compare with 0: PPID is always positive
    cmp r0, #0
    bgt ppid_ok

    // Failed
    mov r0, #1
    mov r7, #1
    svc #0

ppid_ok:
    // Success: exit 0
    mov r0, #0
    mov r7, #1
    svc #0
