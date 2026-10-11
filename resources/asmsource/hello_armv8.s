.global _start
.text

_start:
    mov x0, #1           // fd = stdout
    ldr x1, =msg         // buffer pointer
    mov x2, #26          // length
    mov x8, #64          // Linux AArch64 sys_write (64)
    svc #0

    mov x0, #0           // exit code 0
    mov x8, #93          // Linux AArch64 sys_exit (93)
    svc #0

.data
msg:
    .ascii "Hello from IVM ARMv8 VM!\n"
