.global _start
.text

_start:
    mov x0, #0           // sum = 0
    mov x1, #1           // i = 1
loop:
    cmp x1, #10
    b.gt done
    add x0, x0, x1
    add x1, x1, #1
    b loop
done:
    mov x8, #93          // sys_exit(55)
    svc #0
