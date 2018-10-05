// +build amd64

// fast scanning in assembly is designed around the SSE4v2 PCMPISTRI instruction,
// which (in one of its operational modes) takes an up to 16 byte vector of bytes.
// Each pair comprises a value range.  scanning proceeds until it finds a byte
// inside this value range.

// FIXME: Detect presence of SSE4.
TEXT ·hasAsm(SB),$0-1
    MOVQ $1, AX
    CPUID
    SHRQ $23, CX
    ANDQ $1, CX
    MOVB CX, ret+0(FP)
RET

TEXT ·scanNumberChars(SB),4,$0-40
    // load range 0-9
    MOVQ $0x000000FF3a2F01, BX
    MOVQ BX, X0
    MOVQ $4, AX

    // first argument is the byte array we're scanning
    MOVQ s+0(FP), SI
    MOVQ offset+24(FP), BX
    ADDQ BX, SI

    // second argument is the offset into the array
    // now load the string length into AX, loop control
    // we'll do one loop for every substring of up to 16 bytes,
    // thats (x + 15) >> 4
    MOVQ s_len+8(FP), DX
    SUBQ BX, DX

    // AX holds $6 - len of our range (argument to PCMPESTRI)
    // X0 holds numeric byte ranges that should cause us to halt scanning
    // DX holds the number of bytes remaining (argument to PCMPESTRI)
    // SI holds the memory address of slice + offset
    // BX holds the number of bytes processed

    MOVQ $0, BX
    TESTQ DX,DX
    JZ scanNumberCharsEnd
scanNumberCharsLoop:
    PCMPESTRI $0x04, 0(SI), X0
    JC scanNumberCharsFound
    CMPQ DX, $16
    JLE scanNumberCharsEnd
    ADDQ $16, SI
    ADDQ $16, BX
    SUBQ $16, DX
    JMP scanNumberCharsLoop

scanNumberCharsEnd:
    ADDQ DX, BX
    MOVQ BX, ret+32(FP)
    RET

scanNumberCharsFound:
    ADDQ CX, BX
    MOVQ BX, ret+32(FP)
    RET

TEXT ·scanNonSpecialStringChars(SB),4,$0-40
    // load range (control, '"', and '\')
    MOVQ $0x5c5c22221f01, BX
    MOVQ BX, X0
    MOVQ $6, AX

    // first argument is the byte array we're scanning
    MOVQ s+0(FP), SI
    MOVQ offset+24(FP), BX
    ADDQ BX, SI

    // second argument is the offset into the array
    // now load the string length into AX, loop control
    // we'll do one loop for every substring of up to 16 bytes,
    // thats (x + 15) >> 4
    MOVQ s_len+8(FP), DX
    SUBQ BX, DX

    // AX holds $6 - len of our range (argument to PCMPESTRI)
    // X0 holds numeric byte ranges that should cause us to halt scanning
    // DX holds the number of bytes remaining (argument to PCMPESTRI)
    // SI holds the memory address of slice + offset
    // BX holds the number of bytes processed

    MOVQ $0, BX
    TESTQ DX,DX
    JZ scanNonSpecialStringCharsEnd
scanNonSpecialStringCharsLoop:
    PCMPESTRI $0x04, 0(SI), X0
    JC scanNonSpecialStringCharsFound
    CMPQ DX, $16
    JLE scanNonSpecialStringCharsEnd
    ADDQ $16, SI
    ADDQ $16, BX
    SUBQ $16, DX
    JMP scanNonSpecialStringCharsLoop

scanNonSpecialStringCharsEnd:
    ADDQ DX, BX
    MOVQ BX, ret+32(FP)
    RET

scanNonSpecialStringCharsFound:
    ADDQ CX, BX
    MOVQ BX, ret+32(FP)
    RET
