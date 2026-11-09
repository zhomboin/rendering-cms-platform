package imaging

import (
	"bytes"
	"image/jpeg"
	"testing"
)

func TestJPEGStripsMetadataAfterScanAndTrailer(t *testing.T) {
	base := encodeJPEG(t, 4, 4)
	for _, marker := range []byte{0xE1, 0xE5, 0xED, 0xEF, 0xFE} {
		payload := []byte("Exif\x00\x00private GPS and XMP")
		input := append([]byte{}, base[:len(base)-2]...)
		input = append(input, jpegSegment(marker, payload)...)
		input = append(input, base[len(base)-2:]...)
		if _, err := jpeg.Decode(bytes.NewReader(input)); err != nil {
			t.Fatalf("fixture is not a decodable JPEG: %v", err)
		}
		input = append(input, []byte("private appended trailer")...)
		result, changed, err := StripMetadata(input, TypeJPEG)
		if err != nil || !changed || !bytes.Equal(result, base) {
			t.Fatalf("marker %x: changed=%v error=%v, metadata or trailer retained", marker, changed, err)
		}
		original, _ := jpeg.Decode(bytes.NewReader(base))
		decoded, err := jpeg.Decode(bytes.NewReader(result))
		if err != nil || decoded.Bounds() != original.Bounds() {
			t.Fatalf("decode cleaned JPEG: %v", err)
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				r1, g1, b1, a1 := original.At(x, y).RGBA()
				r2, g2, b2, a2 := decoded.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Fatal("metadata stripping changed pixel values")
				}
			}
		}
	}
}

func TestJPEGRejectsMissingEndAndTruncatedMarker(t *testing.T) {
	base := encodeJPEG(t, 2, 2)
	inputs := [][]byte{
		base[:len(base)-2],
		append(append([]byte{}, base[:len(base)-2]...), 0xff),
		append(append([]byte{}, base[:len(base)-2]...), 0xff, 0xe1, 0, 9, 1),
		{0xff, 0xd8, 0xff, 0x00, 0xff, 0xd9},
	}
	for _, input := range inputs {
		if _, _, err := StripMetadata(input, TypeJPEG); err == nil {
			t.Fatalf("accepted truncated or malformed JPEG (%d bytes)", len(input))
		}
	}
}

func progressiveJPEGFixture() []byte {
	result := []byte{0xff, 0xd8}
	quantization := append([]byte{0}, bytes.Repeat([]byte{1}, 64)...)
	result = append(result, jpegSegment(0xdb, quantization)...)
	result = append(result, jpegSegment(0xc2, []byte{8, 0, 8, 0, 8, 1, 1, 0x11, 0})...)
	for _, table := range []byte{0x00, 0x10} {
		huffman := append([]byte{table, 1}, make([]byte, 15)...)
		huffman = append(huffman, 0)
		result = append(result, jpegSegment(0xc4, huffman)...)
	}
	result = append(result, jpegSegment(0xda, []byte{1, 1, 0, 0, 0, 0})...)
	result = append(result, 0x7f)
	result = append(result, jpegSegment(0xda, []byte{1, 1, 0, 1, 63, 0})...)
	return append(result, 0x7f, 0xff, 0xd9)
}

func TestProgressiveJPEGRemovesMetadataBetweenScansWithoutChangingPixels(t *testing.T) {
	base := progressiveJPEGFixture()
	if _, err := jpeg.Decode(bytes.NewReader(base)); err != nil {
		t.Fatalf("invalid progressive fixture: %v", err)
	}
	second := bytes.LastIndex(base, []byte{0xff, 0xda})
	input := append([]byte{}, base[:second]...)
	input = append(input, jpegSegment(0xe1, []byte("Exif\x00\x00private GPS"))...)
	input = append(input, jpegSegment(0xfe, []byte("private comment"))...)
	input = append(input, base[second:]...)
	if _, err := jpeg.Decode(bytes.NewReader(input)); err != nil {
		t.Fatalf("metadata fixture must remain decodable: %v", err)
	}
	cleaned, changed, err := StripMetadata(input, TypeJPEG)
	if err != nil || !changed || !bytes.Equal(cleaned, base) {
		t.Fatalf("progressive scans changed or metadata survived: %v", err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(cleaned)); err != nil {
		t.Fatal(err)
	}
}

func TestJPEGScanEndPreservesStuffingAndRestartMarkers(t *testing.T) {
	scan := []byte{1, 0xff, 0, 2, 0xff, 0xd0, 3, 0xff, 0xff, 0xd7, 4, 0xff, 0x01, 5}
	data := append(append([]byte{}, scan...), 0xff, 0xff, 0xe1, 0, 2)
	end, ok := jpegScanEnd(data, 0)
	if !ok || end != len(scan) {
		t.Fatalf("scan end=%d ok=%v", end, ok)
	}
	for _, input := range [][]byte{scan, append(scan, 0xff), append(scan, 0xff, 0xff)} {
		if _, ok := jpegScanEnd(input, 0); ok {
			t.Fatal("truncated scan accepted")
		}
	}
}
