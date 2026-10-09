package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"ivm/src/isa"
)

var (
	ErrInvalidMagic    = errors.New("invalid .ivm magic header")
	ErrUnsupportedArch = errors.New("unsupported architecture in IVM file")
)

const ArchARMv7 uint16 = 0x0001

type Header struct {
	Magic        [4]byte
	Arch         uint16
	Flags        uint16
	EntryPC      uint32
	CodeLoadAddr uint32
	CodeSize     uint32
	RawCodeSize  uint32
	DataLoadAddr uint32
	DataSize     uint32
	BSSSize      uint32
	StackSize    uint32
}

type IVMFile struct {
	Header   Header
	Bytecode []byte
	RawCode  []byte
	Data     []byte
}

// SaveToFile writes the IVM container to disk.
func (f *IVMFile) SaveToFile(filepath string) error {
	buf := new(bytes.Buffer)

	f.Header.Magic = isa.IVMMagic
	f.Header.CodeSize = uint32(len(f.Bytecode))
	f.Header.RawCodeSize = uint32(len(f.RawCode))
	f.Header.DataSize = uint32(len(f.Data))

	if err := binary.Write(buf, binary.LittleEndian, &f.Header); err != nil {
		return fmt.Errorf("failed to write IVM header: %w", err)
	}

	if len(f.Bytecode) > 0 {
		if _, err := buf.Write(f.Bytecode); err != nil {
			return fmt.Errorf("failed to write bytecode payload: %w", err)
		}
	}

	if len(f.RawCode) > 0 {
		if _, err := buf.Write(f.RawCode); err != nil {
			return fmt.Errorf("failed to write raw code payload: %w", err)
		}
	}

	if len(f.Data) > 0 {
		if _, err := buf.Write(f.Data); err != nil {
			return fmt.Errorf("failed to write data payload: %w", err)
		}
	}

	return os.WriteFile(filepath, buf.Bytes(), 0644)
}

// LoadFromFile reads an IVM container from disk.
func LoadFromFile(filepath string) (*IVMFile, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	r := bytes.NewReader(data)
	var f IVMFile

	if err := binary.Read(r, binary.LittleEndian, &f.Header); err != nil {
		return nil, fmt.Errorf("corrupt IVM header: %w", err)
	}

	if f.Header.Magic != isa.IVMMagic {
		return nil, ErrInvalidMagic
	}

	if f.Header.Arch != ArchARMv7 {
		return nil, fmt.Errorf("%w: 0x%04X", ErrUnsupportedArch, f.Header.Arch)
	}

	if f.Header.CodeSize > 0 {
		f.Bytecode = make([]byte, f.Header.CodeSize)
		if _, err := io.ReadFull(r, f.Bytecode); err != nil {
			return nil, fmt.Errorf("failed to read bytecode payload: %w", err)
		}
	}

	if f.Header.RawCodeSize > 0 {
		f.RawCode = make([]byte, f.Header.RawCodeSize)
		if _, err := io.ReadFull(r, f.RawCode); err != nil {
			return nil, fmt.Errorf("failed to read raw code payload: %w", err)
		}
	}

	if f.Header.DataSize > 0 {
		f.Data = make([]byte, f.Header.DataSize)
		if _, err := io.ReadFull(r, f.Data); err != nil {
			return nil, fmt.Errorf("failed to read data payload: %w", err)
		}
	}

	return &f, nil
}
