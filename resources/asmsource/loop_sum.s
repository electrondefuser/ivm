.syntax unified
.global _start
.text

_start:
    mov r0, #0           @ accumulator sum = 0
    mov r1, #10          @ counter i = 10

loop:
    add r0, r0, r1       @ sum += i
    subs r1, r1, #1      @ i-- and update flags
    bne loop             @ if i != 0 goto loop

    mov r7, #1           @ sys_exit (exit status in r0 = 55)
    svc #0
