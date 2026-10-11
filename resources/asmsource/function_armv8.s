.global _start
.text

add_func:
    stp x29, x30, [sp, #-16]!
    add x0, x0, x1
    ldp x29, x30, [sp], #16
    ret

_start:
    mov x0, #20
    mov x1, #22
    bl add_func          // x0 = 42
    mov x8, #93          // sys_exit
    svc #0
