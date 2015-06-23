// +build amd64

// fast scanning in assembly is designed around the SSE4v2 PCMPISTRI instruction,
// which (in one of its operational modes) takes an up to 16 byte vector of bytes.
// Each pair comprises a value range.  scanning proceeds until it finds a byte
// inside this value range.

// XXX: fixme
TEXT ·hasAsm(SB),4,$0
    MOVQ $1, AX
    CPUID
    SHRQ $23, CX
    ANDQ $1, CX
    MOVB CX, ret+0(FP)
RET

TEXT ·scanNumberChars(SB),4,$0-40
    // load range (0-9)
    MOVQ $0x000000FF3a2F01, BX
    MOVQ BX, X0
    // first argument is the byte array we're scanning
    MOVQ s+0(FP), SI
    MOVQ offset+24(FP), BX
    ADDQ BX, SI

    // second argument is the offset into the array
    // now load the string length into AX, loop control
    // we'll do one loop for every substring of up to 16 bytes,
    // thats (x + 15) >> 4
	MOVQ s+8(FP), AX
    SUBQ BX, AX
    ADDQ $15, AX
    SHRQ $4, AX

	// SI holds the memory address of slice + offset
    // AX holds the number of iterations required to process the entire array
    // X0 holds numeric byte ranges that should cause us to halt scanning

    TESTQ AX,AX
    JZ scanNumberCharsEnd
    MOVQ $0, BX
scanNumberCharsLoop:
    // PCMPISTRI $0x24, 0(SI), X0
    BYTE $0x66 ;BYTE $0x0F;	BYTE $0x3A;	BYTE $0x63 ; BYTE $0x06 ; BYTE $0x24
    JC scanNumberCharsFound
    ADDQ $16, SI
    ADDQ $1, BX
    DECQ AX
    TESTQ AX,AX
    JNZ scanNumberCharsLoop

scanNumberCharsEnd:
    MOVQ $0, ret+32(FP)
    RET

scanNumberCharsFound:
    SHLQ $4, BX
    ADDQ CX, BX
    MOVQ BX, ret+32(FP)
    RET

TEXT ·scanNonSpecialStringChars(SB),4,$0-40
    // load range (control, '"', and '\')
    MOVQ $0x5c5c22221f01, BX
    MOVQ BX, X0
    // first argument is the byte array we're scanning
    MOVQ s+0(FP), SI
    MOVQ offset+24(FP), BX
    ADDQ BX, SI

    // second argument is the offset into the array
    // now load the string length into AX, loop control
    // we'll do one loop for every substring of up to 16 bytes,
    // thats (x + 15) >> 4
	MOVQ s+8(FP), AX
    SUBQ BX, AX
    ADDQ $15, AX
    SHRQ $4, AX

	// SI holds the memory address of slice + offset
    // AX holds the number of iterations required to process the entire array
    // X0 holds numeric byte ranges that should cause us to halt scanning

    TESTQ AX,AX
    JZ scanNonSpecialStringCharsEnd
    MOVQ $0, BX
scanNonSpecialStringCharsLoop:
    // PCMPISTRI $0x24, 0(SI), X0
    BYTE $0x66 ;BYTE $0x0F;	BYTE $0x3A;	BYTE $0x63 ; BYTE $0x06 ; BYTE $0x24
    JC scanNonSpecialStringCharsFound
    ADDQ $16, SI
    ADDQ $1, BX
    DECQ AX
    TESTQ AX,AX
    JNZ scanNonSpecialStringCharsLoop

scanNonSpecialStringCharsEnd:
    MOVQ $0, ret+32(FP)
    RET

scanNonSpecialStringCharsFound:
    SHLQ $4, BX
    ADDQ CX, BX
    MOVQ BX, ret+32(FP)
    RET


// NOTE: as implemented, the overhead of this routine for terse json outwieghs the
// benefit
TEXT ·scanWhitespaceChars(SB),4,$0-40
    // load range (control, '"', and '\')
    MOVQ $0xff211f0e0801, BX
    MOVQ BX, X0
    // first argument is the byte array we're scanning
    MOVQ s+0(FP), SI
    MOVQ offset+24(FP), BX
    ADDQ BX, SI

    // second argument is the offset into the array
    // now load the string length into AX, loop control
    // we'll do one loop for every substring of up to 16 bytes,
    // thats (x + 15) >> 4
	MOVQ s+8(FP), AX
    SUBQ BX, AX
    ADDQ $15, AX
    SHRQ $4, AX

	// SI holds the memory address of slice + offset
    // AX holds the number of iterations required to process the entire array
    // X0 holds numeric byte ranges that should cause us to halt scanning

    TESTQ AX,AX
    JZ scanWhitespaceCharsEnd
    MOVQ $0, BX
scanWhitespaceCharsLoop:
    // PCMPISTRI $0x24, 0(SI), X0
    BYTE $0x66 ;BYTE $0x0F;	BYTE $0x3A;	BYTE $0x63 ; BYTE $0x06 ; BYTE $0x24
    JC scanWhitespaceCharsFound
    ADDQ $16, SI
    ADDQ $1, BX
    DECQ AX
    TESTQ AX,AX
    JNZ scanWhitespaceCharsLoop

scanWhitespaceCharsEnd:
    MOVQ $0, ret+32(FP)
    RET

scanWhitespaceCharsFound:
    SHLQ $4, BX
    ADDQ CX, BX
    MOVQ BX, ret+32(FP)
    RET
