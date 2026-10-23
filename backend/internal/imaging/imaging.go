// Package imaging 提供相册照片上传所需的服务端图片处理：
// 魔数嗅探、像素尺寸提取（权威值）、EXIF GPS 剥离与拍摄时间提取。
// 只支持 png / jpeg / webp 三种照片格式，全部纯 Go 实现，不做像素重编码。
package imaging

import (
	"bytes"
	"errors"
	"image/jpeg"
	"image/png"
)

const (
	TypeJPEG = "image/jpeg"
	TypePNG  = "image/png"
	TypeWebP = "image/webp"
)

var (
	ErrUnknownFormat = errors.New("imaging: 未识别的图片格式")
	ErrCorrupted     = errors.New("imaging: 图片数据损坏")
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// Sniff 依据魔数判断图片内容类型，不信任客户端声明的 Content-Type。
func Sniff(data []byte) (string, bool) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return TypeJPEG, true
	case bytes.HasPrefix(data, pngSignature):
		return TypePNG, true
	case isWebP(data):
		return TypeWebP, true
	default:
		return "", false
	}
}

func isWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

// Dimensions 返回图片的像素尺寸，与实际像素一致，作为接口的权威值。
func Dimensions(data []byte) (int, int, error) {
	contentType, ok := Sniff(data)
	if !ok {
		return 0, 0, ErrUnknownFormat
	}
	switch contentType {
	case TypePNG:
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, ErrCorrupted
		}
		return config.Width, config.Height, nil
	case TypeJPEG:
		config, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, ErrCorrupted
		}
		return config.Width, config.Height, nil
	case TypeWebP:
		return webpDimensions(data)
	default:
		return 0, 0, ErrUnknownFormat
	}
}

// webpDimensions 按 RIFF chunk 结构解析：优先 VP8X 画布尺寸，
// 否则解析首个 VP8（有损）或 VP8L（无损）帧头。
func webpDimensions(data []byte) (int, int, error) {
	width, height := 0, 0
	found := false
	pos := 12
	for pos+8 <= len(data) {
		fourCC := string(data[pos : pos+4])
		size := int(uint32(data[pos+4]) | uint32(data[pos+5])<<8 | uint32(data[pos+6])<<16 | uint32(data[pos+7])<<24)
		body := pos + 8
		padded := size + (size & 1)
		if size < 0 || body+padded > len(data) {
			return 0, 0, ErrCorrupted
		}
		switch fourCC {
		case "VP8X":
			// 4 字节标志位 + 3 字节宽 - 1 + 3 字节高 - 1（小端）。
			if size >= 10 {
				width = int(data[body+4]) | int(data[body+5])<<8 | int(data[body+6])<<16
				height = int(data[body+7]) | int(data[body+8])<<8 | int(data[body+9])<<16
				width++
				height++
				found = true
			}
		case "VP8 ":
			// 3 字节帧 tag + 3 字节起始码 9D 01 2A + 2 字节宽 + 2 字节高（低 14 位有效）。
			if !found && size >= 10 &&
				data[body+3] == 0x9D && data[body+4] == 0x01 && data[body+5] == 0x2A {
				width = (int(data[body+6]) | int(data[body+7])<<8) & 0x3FFF
				height = (int(data[body+8]) | int(data[body+9])<<8) & 0x3FFF
				found = true
			}
		case "VP8L":
			// 1 字节签名 0x2F + 32 位小端位流：低 14 位宽 - 1，随后 14 位高 - 1。
			if !found && size >= 5 && data[body] == 0x2F {
				bits := uint32(data[body+1]) | uint32(data[body+2])<<8 | uint32(data[body+3])<<16 | uint32(data[body+4])<<24
				width = int(bits&0x3FFF) + 1
				height = int((bits>>14)&0x3FFF) + 1
				found = true
			}
		}
		if found {
			return width, height, nil
		}
		pos = body + padded
	}
	return 0, 0, ErrCorrupted
}
