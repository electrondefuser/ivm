.syntax unified
.global _start
.text

_start:
    mov r0, #15
    mov r1, #27
    add r0, r0, r1       @ r0 = 15 + 27 = 42
    mov r7, #1          @ sys_exit
    svc #0
