package vm

import (
	"fmt"
)

const PageSize = 65536 // 64 KB Pages

type Memory struct {
	pages map[uint16][]byte
	Break uint32 // Heap break address (sys_brk)
}

func NewMemory() *Memory {
	return &Memory{
		pages: make(map[uint16][]byte),
		Break: 0x00030000,
	}
}

func (m *Memory) getPage(pageIdx uint16, create bool) []byte {
	page, exists := m.pages[pageIdx]
	if !exists && create {
		page = make([]byte, PageSize)
		m.pages[pageIdx] = page
	}
	return page
}

func (m *Memory) Read8(addr uint32) uint8 {
	pageIdx := uint16(addr >> 16)
	offset := uint16(addr & 0xFFFF)
	page := m.getPage(pageIdx, false)
	if page == nil {
		return 0
	}
	return page[offset]
}

func (m *Memory) Write8(addr uint32, val uint8) {
	pageIdx := uint16(addr >> 16)
	offset := uint16(addr & 0xFFFF)
	page := m.getPage(pageIdx, true)
	page[offset] = val
}

func (m *Memory) Read16(addr uint32) uint16 {
	b0 := uint16(m.Read8(addr))
	b1 := uint16(m.Read8(addr + 1))
	return b0 | (b1 << 8)
}

func (m *Memory) Write16(addr uint32, val uint16) {
	m.Write8(addr, uint8(val&0xFF))
	m.Write8(addr+1, uint8((val>>8)&0xFF))
}

func (m *Memory) Read32(addr uint32) uint32 {
	b0 := uint32(m.Read8(addr))
	b1 := uint32(m.Read8(addr + 1))
	b2 := uint32(m.Read8(addr + 2))
	b3 := uint32(m.Read8(addr + 3))
	return b0 | (b1 << 8) | (b2 << 16) | (b3 << 24)
}

func (m *Memory) Write32(addr uint32, val uint32) {
	m.Write8(addr, uint8(val&0xFF))
	m.Write8(addr+1, uint8((val>>8)&0xFF))
	m.Write8(addr+2, uint8((val>>16)&0xFF))
	m.Write8(addr+3, uint8((val>>24)&0xFF))
}

func (m *Memory) ReadBytes(addr uint32, length uint32) []byte {
	res := make([]byte, length)

	for i := range length {
		res[i] = m.Read8(addr + i)
	}
	return res
}

func (m *Memory) WriteBytes(addr uint32, data []byte) {
	for i, b := range data {
		m.Write8(addr+uint32(i), b)
	}
}

func (m *Memory) ReadString(addr uint32) string {
	var bytes []byte
	curr := addr
	for {
		b := m.Read8(curr)
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
		curr++
	}
	return string(bytes)
}

func (m *Memory) Push32(sp *uint32, val uint32) {
	*sp -= 4
	m.Write32(*sp, val)
}

func (m *Memory) Pop32(sp *uint32) uint32 {
	val := m.Read32(*sp)
	*sp += 4
	return val
}

func (m *Memory) LoadSegment(startAddr uint32, data []byte) {
	m.WriteBytes(startAddr, data)
}

func (m *Memory) DumpHex(startAddr uint32, length uint32) string {
	out := ""
	for i := uint32(0); i < length; i += 16 {
		out += fmt.Sprintf("0x%08X: ", startAddr+i)
		for j := uint32(0); j < 16; j++ {
			if i+j < length {
				out += fmt.Sprintf("%02X ", m.Read8(startAddr+i+j))
			} else {
				out += "   "
			}
		}
		out += "\n"
	}
	return out
}
