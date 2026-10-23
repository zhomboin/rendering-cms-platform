package imaging

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"
)

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x7F, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(y), G: uint8(x), B: 0x33, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// buildEXIFTIFF 构造仅含 DateTimeOriginal 的最小 EXIF 载荷（小端 TIFF）。
func buildEXIFTIFF(dateTime string) []byte {
	var buf bytes.Buffer
	buf.WriteString("II")
	_ = binary.Write(&buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(8))
	// IFD0：单条目指向 Exif 子 IFD。
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(exifTagExifIFDPointer))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(4))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(26))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	// Exif 子 IFD：单条目 DateTimeOriginal。
	exifOffset := uint32(44)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(exifTagDateTimeOriginal))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(exifTypeASCII))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(dateTime)+1))
	_ = binary.Write(&buf, binary.LittleEndian, exifOffset)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	buf.WriteString(dateTime)
	buf.WriteByte(0)
	return buf.Bytes()
}

func jpegSegment(marker byte, payload []byte) []byte {
	segment := make([]byte, 0, 4+len(payload))
	segment = append(segment, 0xFF, marker, byte((len(payload)+2)>>8), byte(len(payload)+2))
	segment = append(segment, payload...)
	return segment
}

func jpegWithSegments(base []byte, segments ...[]byte) []byte {
	out := make([]byte, 0, len(base))
	out = append(out, base[0], base[1])
	for _, segment := range segments {
		out = append(out, segment...)
	}
	out = append(out, base[2:]...)
	return out
}

func jpegWithEXIF(t *testing.T, base []byte, tiff []byte) []byte {
	t.Helper()
	return jpegWithSegments(base, jpegSegment(0xE1, append([]byte("Exif\x00\x00"), tiff...)))
}

func jpegXmppPayload(t *testing.T) []byte {
	t.Helper()
	return append([]byte("http://ns.adobe.com/xap/1.0/\x00"), []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/></x:xmpmeta>`)...)
}

func pngChunk(chunkType string, payload []byte) []byte {
	chunk := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(chunk[0:4], uint32(len(payload)))
	copy(chunk[4:8], chunkType)
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[8+len(payload):], crc32.ChecksumIEEE(chunk[4:8+len(payload)]))
	return chunk
}

func pngWithChunks(base []byte, chunks ...[]byte) []byte {
	out := make([]byte, 0, len(base))
	out = append(out, base[:len(base)-12]...)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	out = append(out, base[len(base)-12:]...)
	return out
}

func pngWithEXIF(t *testing.T, base []byte, payload []byte) []byte {
	t.Helper()
	return pngWithChunks(base, pngChunk("eXIf", payload))
}

func webpChunk(fourCC string, payload []byte) []byte {
	chunk := make([]byte, 8+len(payload)+(len(payload)&1))
	copy(chunk, fourCC)
	binary.LittleEndian.PutUint32(chunk[4:8], uint32(len(payload)))
	copy(chunk[8:], payload)
	return chunk
}

func webpFile(chunks ...[]byte) []byte {
	body := append([]byte("WEBP"), bytes.Join(chunks, nil)...)
	out := make([]byte, 8+len(body))
	copy(out, "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}

func vp8LPayload(width, height int) []byte {
	bits := uint32(width-1) | uint32(height-1)<<14
	return []byte{0x2F, byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24), 0, 0}
}

// vp8XPayload 的 flags 直接写入 VP8X 第一个字节：EXIF 位 0x08、XMP 位 0x04。
func vp8XPayload(width, height int, flags byte) []byte {
	payload := make([]byte, 10)
	payload[0] = flags
	w := uint32(width - 1)
	payload[4] = byte(w)
	payload[5] = byte(w >> 8)
	payload[6] = byte(w >> 16)
	h := uint32(height - 1)
	payload[7] = byte(h)
	payload[8] = byte(h >> 8)
	payload[9] = byte(h >> 16)
	return payload
}

func TestSniff(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", encodePNG(t, 2, 2), TypePNG},
		{"jpeg", encodeJPEG(t, 2, 2), TypeJPEG},
		{"webp", webpFile(webpChunk("VP8L", vp8LPayload(4, 4))), TypeWebP},
		{"text", []byte("hello world"), ""},
		{"truncated", []byte{0xFF, 0xD8}, ""},
	}
	for _, tc := range cases {
		got, ok := Sniff(tc.data)
		if tc.want == "" {
			if ok {
				t.Fatalf("%s: sniff should fail, got %q", tc.name, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("%s: sniff = %q ok=%v, want %q", tc.name, got, ok, tc.want)
		}
	}
}

func TestDimensions(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		width  int
		height int
	}{
		{"png", encodePNG(t, 7, 4), 7, 4},
		{"jpeg", encodeJPEG(t, 5, 3), 5, 3},
		{"webp-vp8l", webpFile(webpChunk("VP8L", vp8LPayload(100, 80))), 100, 80},
		{"webp-vp8x", webpFile(
			webpChunk("VP8X", vp8XPayload(320, 240, 0)),
			webpChunk("VP8L", vp8LPayload(320, 240)),
		), 320, 240},
	}
	for _, tc := range cases {
		width, height, err := Dimensions(tc.data)
		if err != nil {
			t.Fatalf("%s: dimensions: %v", tc.name, err)
		}
		if width != tc.width || height != tc.height {
			t.Fatalf("%s: dimensions = %dx%d, want %dx%d", tc.name, width, height, tc.width, tc.height)
		}
	}
}

func TestDimensionsRejectsCorruptedWebP(t *testing.T) {
	data := webpFile(webpChunk("VP8L", []byte{0x2F, 0x01}))
	if _, _, err := Dimensions(data); err == nil {
		t.Fatal("corrupted webp should fail dimension parsing")
	}
}

func TestStripJPEGMetadata(t *testing.T) {
	base := encodeJPEG(t, 4, 4)
	withEXIF := jpegWithEXIF(t, base, buildEXIFTIFF("2026:10:01 18:42:00"))

	stripped, changed, err := StripMetadata(withEXIF, TypeJPEG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for jpeg with exif")
	}
	if !bytes.Equal(stripped, base) {
		t.Fatal("stripped jpeg should equal original encoded bytes")
	}
	if _, ok := exifPayload(stripped, TypeJPEG); ok {
		t.Fatal("stripped jpeg should not contain exif payload")
	}

	unchanged, changed, err := StripMetadata(base, TypeJPEG)
	if err != nil {
		t.Fatalf("strip clean: %v", err)
	}
	if changed || !bytes.Equal(unchanged, base) {
		t.Fatal("jpeg without exif should stay unchanged")
	}
}

func TestStripJPEGEXIFRejectsCorrupted(t *testing.T) {
	corrupted := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("truncated")...)
	if _, _, err := StripMetadata(corrupted, TypeJPEG); err == nil {
		t.Fatal("corrupted jpeg should fail strip")
	}
}

func TestStripPNGMetadata(t *testing.T) {
	base := encodePNG(t, 3, 2)
	withEXIF := pngWithEXIF(t, base, buildEXIFTIFF("2026:09:30 08:00:00"))

	stripped, changed, err := StripMetadata(withEXIF, TypePNG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for png with exif")
	}
	if !bytes.Equal(stripped, base) {
		t.Fatal("stripped png should equal original encoded bytes")
	}

	unchanged, changed, err := StripMetadata(base, TypePNG)
	if err != nil {
		t.Fatalf("strip clean: %v", err)
	}
	if changed || !bytes.Equal(unchanged, base) {
		t.Fatal("png without exif should stay unchanged")
	}
}

func TestStripWebPMetadata(t *testing.T) {
	withEXIF := webpFile(
		webpChunk("VP8X", vp8XPayload(320, 240, 0x08)),
		webpChunk("EXIF", buildEXIFTIFF("2026:10:01 18:42:00")),
		webpChunk("VP8L", vp8LPayload(320, 240)),
	)

	stripped, changed, err := StripMetadata(withEXIF, TypeWebP)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for webp with exif")
	}
	if _, ok := findWebPChunk(stripped, "EXIF"); ok {
		t.Fatal("stripped webp should not contain EXIF chunk")
	}
	vp8x, ok := findWebPChunk(stripped, "VP8X")
	if !ok {
		t.Fatal("stripped webp should keep VP8X chunk")
	}
	if vp8x[0]&0x08 != 0 {
		t.Fatal("VP8X EXIF flag should be cleared")
	}
	if binary.LittleEndian.Uint32(stripped[4:8]) != uint32(len(stripped)-8) {
		t.Fatal("RIFF size should match actual file length")
	}
	width, height, err := Dimensions(stripped)
	if err != nil || width != 320 || height != 240 {
		t.Fatalf("stripped webp dimensions = %dx%d err=%v, want 320x240", width, height, err)
	}

	clean := webpFile(webpChunk("VP8L", vp8LPayload(10, 10)))
	unchanged, changed, err := StripMetadata(clean, TypeWebP)
	if err != nil {
		t.Fatalf("strip clean: %v", err)
	}
	if changed || !bytes.Equal(unchanged, clean) {
		t.Fatal("webp without exif should stay unchanged")
	}
}

func TestTakenAt(t *testing.T) {
	want := time.Date(2026, 10, 1, 18, 42, 0, 0, time.UTC)

	jpegEXIF := jpegWithEXIF(t, encodeJPEG(t, 2, 2), buildEXIFTIFF("2026:10:01 18:42:00"))
	if got, ok := TakenAt(jpegEXIF, TypeJPEG); !ok || !got.Equal(want) {
		t.Fatalf("jpeg takenAt = %v ok=%v, want %v", got, ok, want)
	}

	pngEXIF := pngWithEXIF(t, encodePNG(t, 2, 2), buildEXIFTIFF("2026:10:01 18:42:00"))
	if got, ok := TakenAt(pngEXIF, TypePNG); !ok || !got.Equal(want) {
		t.Fatalf("png takenAt = %v ok=%v, want %v", got, ok, want)
	}

	webpEXIF := webpFile(
		webpChunk("VP8X", vp8XPayload(4, 4, 0x08)),
		webpChunk("EXIF", buildEXIFTIFF("2026:10:01 18:42:00")),
		webpChunk("VP8L", vp8LPayload(4, 4)),
	)
	if got, ok := TakenAt(webpEXIF, TypeWebP); !ok || !got.Equal(want) {
		t.Fatalf("webp takenAt = %v ok=%v, want %v", got, ok, want)
	}

	if _, ok := TakenAt(encodePNG(t, 2, 2), TypePNG); ok {
		t.Fatal("image without exif should not report takenAt")
	}
}

func TestStripPreservesDimensions(t *testing.T) {
	jpegEXIF := jpegWithEXIF(t, encodeJPEG(t, 9, 6), buildEXIFTIFF("2026:01:02 03:04:05"))
	stripped, _, err := StripMetadata(jpegEXIF, TypeJPEG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	width, height, err := Dimensions(stripped)
	if err != nil || width != 9 || height != 6 {
		t.Fatalf("stripped dimensions = %dx%d err=%v, want 9x6", width, height, err)
	}
}

func TestStripJPEGMetadataStripsXMPAndVendorSegments(t *testing.T) {
	base := encodeJPEG(t, 4, 4)
	xmp := jpegSegment(0xE1, jpegXmppPayload(t))
	extendedXMP := jpegSegment(0xE1, append([]byte("http://ns.adobe.com/xmp/extension/\x00"), []byte("<xmp-ext>")...))
	irb := jpegSegment(0xED, []byte("Photoshop 3.0\x008BIM\x04\x04misc-irb"))
	vendor := jpegSegment(0xE5, []byte("vendor-private-note"))
	comment := jpegSegment(0xFE, []byte("secret camera note"))
	exif := jpegSegment(0xE1, append([]byte("Exif\x00\x00"), buildEXIFTIFF("2026:10:01 18:42:00")...))
	withMetadata := jpegWithSegments(base, xmp, extendedXMP, irb, vendor, comment, exif)

	stripped, changed, err := StripMetadata(withMetadata, TypeJPEG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for jpeg with xmp metadata")
	}
	if !bytes.Equal(stripped, base) {
		t.Fatal("stripped jpeg should equal original encoded bytes")
	}
}

func TestStripJPEGMetadataKeepsDecodeSegments(t *testing.T) {
	base := encodeJPEG(t, 4, 4)
	icc := jpegSegment(0xE2, append([]byte("ICC_PROFILE\x00\x01"), []byte("fake-icc-payload")...))
	adobe := jpegSegment(0xEE, append([]byte("Adobe\x00"), []byte{100, 0, 0, 0, 0}...))
	xmp := jpegSegment(0xE1, jpegXmppPayload(t))
	withSegments := jpegWithSegments(base, icc, adobe, xmp)
	expected := jpegWithSegments(base, icc, adobe)

	stripped, changed, err := StripMetadata(withSegments, TypeJPEG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed when xmp segment is dropped")
	}
	if !bytes.Equal(stripped, expected) {
		t.Fatal("stripped jpeg should keep APP2 ICC and APP14 Adobe segments")
	}
}

func TestStripPNGMetadataStripsTextChunks(t *testing.T) {
	base := encodePNG(t, 3, 2)
	xmp := pngChunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00<x:xmpmeta/>"))
	title := pngChunk("tEXt", []byte("Title\x00holiday"))
	zcomment := pngChunk("zTXt", []byte("Comment\x00\x00compressed"))
	exif := pngChunk("eXIf", buildEXIFTIFF("2026:09:30 08:00:00"))
	withText := pngWithChunks(base, xmp, title, zcomment, exif)

	stripped, changed, err := StripMetadata(withText, TypePNG)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for png with text metadata")
	}
	if !bytes.Equal(stripped, base) {
		t.Fatal("stripped png should equal original encoded bytes")
	}
}

func TestStripWebPMetadataStripsXMP(t *testing.T) {
	withMetadata := webpFile(
		webpChunk("VP8X", vp8XPayload(320, 240, 0x08|0x04)),
		webpChunk("EXIF", buildEXIFTIFF("2026:10:01 18:42:00")),
		webpChunk("XMP ", []byte("<x:xmpmeta/>")),
		webpChunk("ICCP", []byte("fake-icc")),
		webpChunk("VP8L", vp8LPayload(320, 240)),
	)

	stripped, changed, err := StripMetadata(withMetadata, TypeWebP)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("strip should report changed for webp with xmp chunk")
	}
	if _, ok := findWebPChunk(stripped, "EXIF"); ok {
		t.Fatal("stripped webp should not contain EXIF chunk")
	}
	if _, ok := findWebPChunk(stripped, "XMP "); ok {
		t.Fatal("stripped webp should not contain XMP chunk")
	}
	if _, ok := findWebPChunk(stripped, "ICCP"); !ok {
		t.Fatal("stripped webp should keep ICCP chunk")
	}
	vp8x, ok := findWebPChunk(stripped, "VP8X")
	if !ok {
		t.Fatal("stripped webp should keep VP8X chunk")
	}
	if vp8x[0]&0x0C != 0 {
		t.Fatal("VP8X EXIF and XMP flags should be cleared")
	}
	if binary.LittleEndian.Uint32(stripped[4:8]) != uint32(len(stripped)-8) {
		t.Fatal("RIFF size should match actual file length")
	}
	if width, height, err := Dimensions(stripped); err != nil || width != 320 || height != 240 {
		t.Fatalf("stripped webp dimensions = %dx%d err=%v, want 320x240", width, height, err)
	}
}

func TestStripWebPMetadataClearsStaleVP8XFlags(t *testing.T) {
	staleFlags := webpFile(
		webpChunk("VP8X", vp8XPayload(320, 240, 0x04)),
		webpChunk("VP8L", vp8LPayload(320, 240)),
	)
	stripped, changed, err := StripMetadata(staleFlags, TypeWebP)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if !changed {
		t.Fatal("clearing stale VP8X XMP flag should report changed")
	}
	vp8x, ok := findWebPChunk(stripped, "VP8X")
	if !ok || vp8x[0]&0x04 != 0 {
		t.Fatal("VP8X XMP flag should be cleared")
	}
}
