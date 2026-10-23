package imaging

import (
	"encoding/binary"
	"time"
)

// exifTagDateTimeOriginal 与 exifTagExifIFDPointer 为 EXIF 规范中的标签号。
const (
	exifTagExifIFDPointer    = 0x8769
	exifTagDateTimeOriginal  = 0x9003
	exifTypeASCII            = 2
	exifDateTimeOriginalForm = "2006:01:02 15:04:05"
)

// TakenAt 从图片的 EXIF 元数据中提取拍摄时间（DateTimeOriginal）。
// EXIF 原始时间不含时区，按 UTC 解析；没有 EXIF 或解析失败时返回 ok=false。
func TakenAt(data []byte, contentType string) (time.Time, bool) {
	payload, ok := exifPayload(data, contentType)
	if !ok {
		return time.Time{}, false
	}
	value, ok := exifASCIIValue(payload, exifTagExifIFDPointer, exifTagDateTimeOriginal)
	if !ok {
		return time.Time{}, false
	}
	// 值为 "YYYY:MM:DD HH:MM:SS" 并以 NUL 结尾。
	taken, err := time.Parse(exifDateTimeOriginalForm, trimASCIIZ(value))
	if err != nil {
		return time.Time{}, false
	}
	return taken.UTC(), true
}

func trimASCIIZ(value string) string {
	for len(value) > 0 && (value[len(value)-1] == 0 || value[len(value)-1] == ' ') {
		value = value[:len(value)-1]
	}
	return value
}

// exifASCIIValue 在 TIFF 头定位 IFD0，跳转到 Exif 子 IFD 并按标签号取 ASCII 值。
func exifASCIIValue(payload []byte, ifdPointerTag uint16, wantTag uint16) (string, bool) {
	if len(payload) < 8 {
		return "", false
	}
	var order binary.ByteOrder
	switch string(payload[0:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return "", false
	}
	if order.Uint16(payload[2:4]) != 42 {
		return "", false
	}
	ifd0Offset := int(order.Uint32(payload[4:8]))

	exifIFDOffset, ok := findIFDPointer(payload, order, ifd0Offset, ifdPointerTag)
	if !ok {
		return "", false
	}

	entry, ok := findIFDEntry(payload, order, exifIFDOffset, wantTag)
	if !ok || entry.typeCode != exifTypeASCII {
		return "", false
	}
	if entry.count == 0 || int(entry.count) > len(payload) {
		return "", false
	}
	value := entry.value
	if int(entry.count) > 4 {
		offset := int(entry.countOrOffset)
		if offset+int(entry.count) > len(payload) {
			return "", false
		}
		value = payload[offset : offset+int(entry.count)]
	} else {
		value = value[:entry.count]
	}
	return string(value), true
}

type ifdEntry struct {
	tag           uint16
	typeCode      uint16
	count         uint32
	countOrOffset uint32
	value         []byte
}

// findIFDPointer 遍历 IFD 条目，找到指向子 IFD 的长整型指针标签。
func findIFDPointer(payload []byte, order binary.ByteOrder, ifdOffset int, pointerTag uint16) (int, bool) {
	entry, ok := findIFDEntry(payload, order, ifdOffset, pointerTag)
	if !ok || entry.count != 1 {
		return 0, false
	}
	offset := int(entry.countOrOffset)
	if offset < 8 || offset >= len(payload) {
		return 0, false
	}
	return offset, true
}

func findIFDEntry(payload []byte, order binary.ByteOrder, ifdOffset int, wantTag uint16) (ifdEntry, bool) {
	if ifdOffset < 8 || ifdOffset+2 > len(payload) {
		return ifdEntry{}, false
	}
	entryCount := int(order.Uint16(payload[ifdOffset : ifdOffset+2]))
	cursor := ifdOffset + 2
	for i := 0; i < entryCount; i++ {
		if cursor+12 > len(payload) {
			return ifdEntry{}, false
		}
		tag := order.Uint16(payload[cursor : cursor+2])
		typeCode := order.Uint16(payload[cursor+2 : cursor+4])
		count := order.Uint32(payload[cursor+4 : cursor+8])
		countOrOffset := order.Uint32(payload[cursor+8 : cursor+12])
		if tag == wantTag {
			return ifdEntry{
				tag:           tag,
				typeCode:      typeCode,
				count:         count,
				countOrOffset: countOrOffset,
				value:         payload[cursor+8 : cursor+12],
			}, true
		}
		cursor += 12
	}
	return ifdEntry{}, false
}
