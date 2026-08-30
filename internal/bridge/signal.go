package bridge

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The addon cannot read files, but SetTexture tells it whether one exists, so a finished turn drops a tiny TGA.
func writeSignal(addonDir, epoch string, id int) error {
	root := filepath.Join(addonDir, "signal")
	dir := filepath.Join(root, SignalEpoch(epoch))
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clear signals: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("signal dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(id)+".tga"), tga8x8(), 0o644); err != nil {
		return fmt.Errorf("write signal: %w", err)
	}
	return nil
}

// SignalEpoch strips the dot so the client does not read it as a file extension.
func SignalEpoch(epoch string) string {
	return strings.ReplaceAll(epoch, ".", "_")
}

// tga8x8 is an uncompressed 32-bit true-color TGA, the smallest power-of-two image the 1.12 client accepts.
func tga8x8() []byte {
	const w, h = 8, 8
	head := make([]byte, 18)
	head[2] = 2
	binary.LittleEndian.PutUint16(head[12:], w)
	binary.LittleEndian.PutUint16(head[14:], h)
	head[16] = 32
	head[17] = 8
	pixels := make([]byte, w*h*4)
	for i := range pixels {
		pixels[i] = 0xff
	}
	return append(head, pixels...)
}
