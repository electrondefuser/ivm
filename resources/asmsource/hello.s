.syntax unified
.global _start
.text

_start:
    mov r0, #1           @ fd = stdout
    ldr r1, =msg         @ buffer pointer
    mov r2, #26          @ length
    mov r7, #4           @ sys_write
    svc #0

    mov r0, #0           @ exit code 0
    mov r7, #1           @ sys_exit
    svc #0

.data
msg:
    .ascii "Hello from IVM ARMv7 VM!\n"
