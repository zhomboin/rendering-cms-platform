package imaging

// stripJPEGMetadata 遍历全部扫描段和段间标记，保留熵编码数据及解码参数。
// EXIF、XMP、厂商元数据和注释在任何位置都移除，EOI 后的附加数据也不保留。
func stripJPEGMetadata(data []byte) ([]byte, bool, error) {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return nil, false, ErrCorrupted
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:2]...)
	pos, changed, scanned := 2, false, false
	for pos < len(data) {
		start := pos
		if data[pos] != 0xff {
			return nil, false, ErrCorrupted
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos == len(data) {
			return nil, false, ErrCorrupted
		}
		marker := data[pos]
		pos++
		switch marker {
		case 0xd9:
			if !scanned {
				return nil, false, ErrCorrupted
			}
			out = append(out, data[start:pos]...)
			return out, changed || pos < len(data), nil
		case 0x01:
			out = append(out, data[start:pos]...)
			continue
		case 0x00, 0xd8, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
			return nil, false, ErrCorrupted
		}
		if pos+2 > len(data) {
			return nil, false, ErrCorrupted
		}
		length := int(data[pos])<<8 | int(data[pos+1])
		if length < 2 || length > len(data)-pos {
			return nil, false, ErrCorrupted
		}
		end := pos + length
		if jpegMetadataMarker(marker) {
			changed = true
		} else {
			out = append(out, data[start:end]...)
		}
		pos = end
		if marker == 0xda || marker == 0xdc {
			// SOS 开始新扫描；DNL 可以位于扫描中，其后仍为熵编码数据。
			scanned = true
			next, ok := jpegScanEnd(data, pos)
			if !ok {
				return nil, false, ErrCorrupted
			}
			out = append(out, data[pos:next]...)
			pos = next
		}
	}
	return nil, false, ErrCorrupted
}

func jpegScanEnd(data []byte, pos int) (int, bool) {
	for pos < len(data) {
		if data[pos] != 0xff {
			pos++
			continue
		}
		start := pos
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos == len(data) {
			return 0, false
		}
		marker := data[pos]
		if marker == 0x00 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			pos++
			continue
		}
		return start, true
	}
	return 0, false
}
