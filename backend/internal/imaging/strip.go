package imaging

import (
	"bytes"
	"encoding/binary"
)

// StripEXIF 从图片中删除 EXIF 元数据（含 GPS 等隐私信息），不做像素重编码、无损保持画质。
// 返回处理后的数据；changed 表示是否实际移除了内容。无法剥离时返回错误，调用方应拒绝存储。
func StripEXIF(data []byte, contentType string) ([]byte, bool, error) {
	switch contentType {
	case TypeJPEG:
		return stripJPEGEXIF(data)
	case TypePNG:
		return stripPNGEXIF(data)
	case TypeWebP:
		return stripWebPEXIF(data)
	default:
		return nil, false, ErrUnknownFormat
	}
}

// exifPayload 定位各容器中 EXIF 的 TIFF 载荷字节，供元数据解析使用。
func exifPayload(data []byte, contentType string) ([]byte, bool) {
	switch contentType {
	case TypeJPEG:
		if start, end, ok := findJPEGExifSegment(data); ok {
			return data[start+6 : end], true
		}
		return nil, false
	case TypePNG:
		payload, ok := findPNGChunk(data, "eXIf")
		return payload, ok
	case TypeWebP:
		payload, ok := findWebPChunk(data, "EXIF")
		return payload, ok
	default:
		return nil, false
	}
}

// JPEG：按段结构遍历到 SOS 为止，删除 "Exif\0\0" 开头的 APP1 段，其余字节原样保留。
func stripJPEGEXIF(data []byte) ([]byte, bool, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, false, ErrCorrupted
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[0], data[1])
	changed := false
	pos := 2
	for pos < len(data) {
		if data[pos] != 0xFF {
			return nil, false, ErrCorrupted
		}
		pos++
		// 段标记前允许填充 0xFF。
		for pos < len(data) && data[pos] == 0xFF {
			pos++
		}
		if pos >= len(data) {
			return nil, false, ErrCorrupted
		}
		marker := data[pos]
		pos++
		if marker == 0xD8 || marker == 0xD9 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			// 独立标记：无长度字段。
			out = append(out, 0xFF, marker)
			continue
		}
		if pos+2 > len(data) {
			return nil, false, ErrCorrupted
		}
		segmentLength := int(data[pos])<<8 | int(data[pos+1])
		if segmentLength < 2 || pos+segmentLength > len(data) {
			return nil, false, ErrCorrupted
		}
		if marker == 0xE1 && segmentLength >= 8 && string(data[pos+2:pos+8]) == "Exif\x00\x00" {
			changed = true
		} else {
			out = append(out, 0xFF, marker)
			out = append(out, data[pos:pos+segmentLength]...)
		}
		if marker == 0xDA {
			// SOS 之后是熵编码数据，原样接回并结束。
			out = append(out, data[pos+segmentLength:]...)
			return out, changed, nil
		}
		pos += segmentLength
	}
	return out, changed, nil
}

func findJPEGExifSegment(data []byte) (start int, end int, ok bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, false
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 0, 0, false
		}
		pos++
		for pos < len(data) && data[pos] == 0xFF {
			pos++
		}
		if pos >= len(data) {
			return 0, 0, false
		}
		marker := data[pos]
		pos++
		if marker == 0xD8 || marker == 0xD9 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if pos+2 > len(data) {
			return 0, 0, false
		}
		segmentLength := int(data[pos])<<8 | int(data[pos+1])
		if segmentLength < 2 || pos+segmentLength > len(data) {
			return 0, 0, false
		}
		if marker == 0xE1 && segmentLength >= 8 && string(data[pos+2:pos+8]) == "Exif\x00\x00" {
			return pos + 2, pos + segmentLength, true
		}
		if marker == 0xDA {
			return 0, 0, false
		}
		pos += segmentLength
	}
	return 0, 0, false
}

// PNG：按 chunk 表删除 eXIf chunk，其余 chunk（含 CRC）原样保留。
func stripPNGEXIF(data []byte) ([]byte, bool, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, false, ErrCorrupted
	}
	out := make([]byte, 0, len(data))
	out = append(out, pngSignature...)
	changed := false
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		chunkType := string(data[pos+4 : pos+8])
		// chunk = 长度 4 + 类型 4 + 数据 length + CRC 4。
		total := 12 + length
		if length < 0 || pos+total > len(data) {
			return nil, false, ErrCorrupted
		}
		if chunkType == "eXIf" {
			changed = true
		} else {
			out = append(out, data[pos:pos+total]...)
		}
		pos += total
	}
	if pos != len(data) {
		return nil, false, ErrCorrupted
	}
	return out, changed, nil
}

func findPNGChunk(data []byte, chunkType string) ([]byte, bool) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, false
	}
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if length < 0 || pos+12+length > len(data) {
			return nil, false
		}
		if string(data[pos+4:pos+8]) == chunkType {
			return data[pos+8 : pos+8+length], true
		}
		pos += 12 + length
	}
	return nil, false
}

// WebP：删除 RIFF 容器中的 EXIF chunk，并同步清除 VP8X 标志位中的 EXIF 位、修正 RIFF 总长。
func stripWebPEXIF(data []byte) ([]byte, bool, error) {
	if !isWebP(data) {
		return nil, false, ErrCorrupted
	}
	out := make([]byte, 0, len(data))
	out = append(out, []byte("RIFF")...)
	out = append(out, 0, 0, 0, 0) // RIFF 长度占位，收尾时回填。
	out = append(out, []byte("WEBP")...)
	changed := false
	pos := 12
	for pos+8 <= len(data) {
		fourCC := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		padded := size + (size & 1)
		if size < 0 || pos+8+padded > len(data) {
			return nil, false, ErrCorrupted
		}
		chunk := data[pos : pos+8+padded]
		if fourCC == "EXIF" {
			changed = true
			pos += 8 + padded
			continue
		}
		if fourCC == "VP8X" && size >= 4 && data[pos+8]&0x08 != 0 {
			modified := append([]byte{}, chunk...)
			modified[8] &^= 0x08 // 清除 EXIF 标志位。
			chunk = modified
		}
		out = append(out, chunk...)
		pos += 8 + padded
	}
	if pos != len(data) {
		return nil, false, ErrCorrupted
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, changed, nil
}

func findWebPChunk(data []byte, fourCC string) ([]byte, bool) {
	if !isWebP(data) {
		return nil, false
	}
	pos := 12
	for pos+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if size < 0 || pos+8+size > len(data) {
			return nil, false
		}
		if string(data[pos:pos+4]) == fourCC {
			return data[pos+8 : pos+8+size], true
		}
		pos += 8 + size + (size & 1)
	}
	return nil, false
}
