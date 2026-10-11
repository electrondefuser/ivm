.global _start
.text

_start:
    mov x0, #100
    mov x1, #4
    sdiv x0, x0, x1      // 100 / 4 = 25 (unvirtualized, host CPU fallback bridge)
    mov x8, #93          // sys_exit(25)
    svc #0
