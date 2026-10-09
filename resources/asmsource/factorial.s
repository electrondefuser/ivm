.syntax unified
.global _start
.text

_start:
    mov r0, #5           @ calculate factorial(5)
    bl factorial

    @ r0 now holds factorial result = 120
    mov r7, #1           @ sys_exit
    svc #0

factorial:
    push {r4, lr}
    cmp r0, #1
    ble fact_base

    mov r4, r0
    sub r0, r0, #1
    bl factorial
    mul r0, r4, r0
    pop {r4, lr}
    bx lr

fact_base:
    mov r0, #1
    pop {r4, lr}
    bx lr
