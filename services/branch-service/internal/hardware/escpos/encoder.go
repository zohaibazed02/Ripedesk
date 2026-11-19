package escpos

import (
	"bytes"
	"strings"
)

type PaperWidth int

const (
	Paper80mm PaperWidth = 48 // 48 columns standard 80mm thermal paper
	Paper58mm PaperWidth = 32 // 32 columns compact 58mm thermal paper
)

type Align byte

const (
	AlignLeft   Align = 0
	AlignCenter Align = 1
	AlignRight  Align = 2
)

type DrawerPin byte

const (
	DrawerPin2 DrawerPin = 0
	DrawerPin5 DrawerPin = 1
)

// Encoder builds deterministic, standards-compliant Epson ESC/POS byte frames.
type Encoder struct {
	buf   bytes.Buffer
	width int
}

func NewEncoder(width PaperWidth) *Encoder {
	w := int(width)
	if w <= 0 {
		w = int(Paper80mm)
	}
	return &Encoder{width: w}
}

func (e *Encoder) Width() int { return e.width }

// Init sends ESC @ (initialize printer).
func (e *Encoder) Init() *Encoder {
	e.buf.Write([]byte{0x1B, 0x40})
	return e
}

// CodeTable sends ESC t n (select character code table, e.g., 0 = PC437, 16 = WPC1252).
func (e *Encoder) CodeTable(table byte) *Encoder {
	e.buf.Write([]byte{0x1B, 0x74, table})
	return e
}

// Align sends ESC a n (0=left, 1=center, 2=right).
func (e *Encoder) Align(a Align) *Encoder {
	e.buf.Write([]byte{0x1B, 0x61, byte(a)})
	return e
}

// Bold sends ESC E n (emphasized mode on/off).
func (e *Encoder) Bold(on bool) *Encoder {
	b := byte(0)
	if on {
		b = 1
	}
	e.buf.Write([]byte{0x1B, 0x45, b})
	return e
}

// TextSize sends GS ! n where widthMult and heightMult are 1..8.
func (e *Encoder) TextSize(widthMult, heightMult byte) *Encoder {
	if widthMult < 1 {
		widthMult = 1
	}
	if widthMult > 8 {
		widthMult = 8
	}
	if heightMult < 1 {
		heightMult = 1
	}
	if heightMult > 8 {
		heightMult = 8
	}
	n := ((widthMult - 1) << 4) | (heightMult - 1)
	e.buf.Write([]byte{0x1D, 0x21, n})
	return e
}

// Invert sends GS B n (white/black reverse printing mode).
func (e *Encoder) Invert(on bool) *Encoder {
	b := byte(0)
	if on {
		b = 1
	}
	e.buf.Write([]byte{0x1D, 0x42, b})
	return e
}

func (e *Encoder) Line(text string) *Encoder {
	e.buf.WriteString(text)
	e.buf.WriteByte('\n')
	return e
}

func (e *Encoder) Feed(lines byte) *Encoder {
	e.buf.Write([]byte{0x1B, 0x64, lines})
	return e
}

func (e *Encoder) Rule(ch byte) *Encoder {
	if ch == 0 {
		ch = '-'
	}
	e.buf.WriteString(strings.Repeat(string(ch), e.width))
	e.buf.WriteByte('\n')
	return e
}

// KeyValue formats a two-column line padded to paper width.
func (e *Encoder) KeyValue(left, right string) *Encoder {
	space := e.width - len(left) - len(right)
	if space < 1 {
		maxLeft := e.width - len(right) - 1
		if maxLeft > 0 && len(left) > maxLeft {
			left = left[:maxLeft]
		}
		space = 1
	}
	e.buf.WriteString(left)
	e.buf.WriteString(strings.Repeat(" ", space))
	e.buf.WriteString(right)
	e.buf.WriteByte('\n')
	return e
}

// QRCode emits Epson GS ( k commands to print a Model-2 QR code.
func (e *Encoder) QRCode(data string, moduleSize byte) *Encoder {
	if data == "" {
		return e
	}
	if moduleSize < 2 || moduleSize > 16 {
		moduleSize = 6
	}
	// Select QR model 2
	e.buf.Write([]byte{0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})
	// Set module size
	e.buf.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, moduleSize})
	// Set error correction level M (49)
	e.buf.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x31})
	// Store data in symbol storage area
	storeLen := len(data) + 3
	pL := byte(storeLen & 0xFF)
	pH := byte((storeLen >> 8) & 0xFF)
	e.buf.Write([]byte{0x1D, 0x28, 0x6B, pL, pH, 0x31, 0x50, 0x30})
	e.buf.WriteString(data)
	// Print symbol
	e.buf.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30})
	return e
}

// Cut sends GS V m (partial or full cut with feed).
func (e *Encoder) Cut(partial bool) *Encoder {
	if partial {
		e.buf.Write([]byte{0x1D, 0x56, 0x42, 0x00})
	} else {
		e.buf.Write([]byte{0x1D, 0x56, 0x41, 0x00})
	}
	return e
}

// CashDrawerKick sends ESC p m t1 t2 (RJ11 drawer kick pulse: 25*2ms ON, 250*2ms OFF).
func (e *Encoder) CashDrawerKick(pin DrawerPin) *Encoder {
	e.buf.Write([]byte{0x1B, 0x70, byte(pin), 0x19, 0xFA})
	return e
}

func (e *Encoder) Bytes() []byte {
	out := make([]byte, e.buf.Len())
	copy(out, e.buf.Bytes())
	return out
}
