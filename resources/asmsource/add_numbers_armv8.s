.global _start
.text

_start:
    mov x0, #15
    mov x1, #27
    add x0, x0, x1       // x0 = 15 + 27 = 42
    mov x8, #93          // Linux AArch64 sys_exit (93)
    svc #0
