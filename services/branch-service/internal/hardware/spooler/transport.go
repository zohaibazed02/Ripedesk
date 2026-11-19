package spooler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type PrinterStatus struct {
	Online    bool   `json:"online"`
	PaperOut  bool   `json:"paper_out"`
	CoverOpen bool   `json:"cover_open"`
	RawStatus byte   `json:"raw_status"`
	Detail    string `json:"detail"`
}

type Transport interface {
	Write(ctx context.Context, frame []byte) error
	QueryStatus(ctx context.Context) (PrinterStatus, error)
}

// NetworkTCPTransport writes raw ESC/POS frames to TCP port 9100 and queries DLE EOT status.
type NetworkTCPTransport struct {
	Address      string
	DialTimeout  time.Duration
	WriteTimeout time.Duration
}

func (t *NetworkTCPTransport) Write(ctx context.Context, frame []byte) error {
	dialTimeout := t.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 3 * time.Second
	}
	writeTimeout := t.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 5 * time.Second
	}

	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", t.Address)
	if err != nil {
		return fmt.Errorf("tcp dial %s: %w", t.Address, err)
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("tcp write %s: %w", t.Address, err)
	}
	return nil
}

func (t *NetworkTCPTransport) QueryStatus(ctx context.Context) (PrinterStatus, error) {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", t.Address)
	if err != nil {
		return PrinterStatus{Online: false, Detail: err.Error()}, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	// DLE EOT 1 (printer status) & DLE EOT 2 (offline cause status)
	if _, err := conn.Write([]byte{0x10, 0x04, 0x01, 0x10, 0x04, 0x02}); err != nil {
		return PrinterStatus{Online: false, Detail: err.Error()}, err
	}
	buf := make([]byte, 2)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return PrinterStatus{Online: true, Detail: "connected (no status byte)"}, nil
	}
	offline := (buf[0] & 0x08) != 0
	coverOpen := n > 1 && (buf[1]&0x04) != 0
	paperOut := n > 1 && (buf[1]&0x20) != 0
	return PrinterStatus{
		Online:    !offline,
		CoverOpen: coverOpen,
		PaperOut:  paperOut,
		RawStatus: buf[0],
		Detail:    "ok",
	}, nil
}

// OSRawSpoolerTransport writes raw ESC/POS bytes to a local device path (e.g. /dev/usb/lp0, COM3, or file).
type OSRawSpoolerTransport struct {
	DevicePath string
}

func (t *OSRawSpoolerTransport) Write(_ context.Context, frame []byte) error {
	f, err := os.OpenFile(t.DevicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open raw device %s: %w", t.DevicePath, err)
	}
	defer f.Close()
	if _, err := f.Write(frame); err != nil {
		return fmt.Errorf("write raw device %s: %w", t.DevicePath, err)
	}
	return nil
}

func (t *OSRawSpoolerTransport) QueryStatus(_ context.Context) (PrinterStatus, error) {
	if _, err := os.Stat(t.DevicePath); err != nil {
		return PrinterStatus{Online: false, Detail: err.Error()}, err
	}
	return PrinterStatus{Online: true, Detail: "device path present"}, nil
}

// VirtualMockPrinterTransport captures rendered ESC/POS frames + ASCII previews in memory.
type CapturedFrame struct {
	Raw          []byte
	ASCIIPreview string
	CapturedAt   time.Time
}

type VirtualMockPrinterTransport struct {
	mu      sync.Mutex
	offline bool
	Frames  []CapturedFrame
}

func NewVirtualMockPrinterTransport() *VirtualMockPrinterTransport {
	return &VirtualMockPrinterTransport{}
}

func (v *VirtualMockPrinterTransport) SetOffline(offline bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.offline = offline
}

func (v *VirtualMockPrinterTransport) Write(_ context.Context, frame []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.offline {
		return errors.New("virtual printer offline: cover open or paper out")
	}
	cp := make([]byte, len(frame))
	copy(cp, frame)
	v.Frames = append(v.Frames, CapturedFrame{
		Raw:          cp,
		ASCIIPreview: extractASCIIPreview(cp),
		CapturedAt:   time.Now().UTC(),
	})
	return nil
}

func (v *VirtualMockPrinterTransport) QueryStatus(_ context.Context) (PrinterStatus, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.offline {
		return PrinterStatus{Online: false, PaperOut: true, Detail: "simulated offline"}, errors.New("simulated offline")
	}
	return PrinterStatus{Online: true, Detail: "virtual printer ready"}, nil
}

func extractASCIIPreview(raw []byte) string {
	var b strings.Builder
	for _, c := range raw {
		if c == '\n' || (c >= 0x20 && c <= 0x7E) {
			b.WriteByte(c)
		}
	}
	return b.String()
}
