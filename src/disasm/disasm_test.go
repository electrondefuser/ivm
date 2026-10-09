package disasm_test

import (
	"ivm/src/disasm"
	"ivm/src/format"
	"testing"
)

func TestDisassembleIVM(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		ivmFile *format.IVMFile
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := disasm.DisassembleIVM(tt.ivmFile)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("DisassembleIVM() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("DisassembleIVM() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("DisassembleIVM() = %v, want %v", got, tt.want)
			}
		})
	}
}
