package assets

import "errors"

const MaxUploadBytes = 20 * 1024 * 1024

const (
	StatusActive   = "active"
	StatusArchived = "archived"
	StatusDeleted  = "deleted"
)

var allowedTypes = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
	"application/zip": true,
}

func ValidateUpload(filename string, contentType string, byteSize int) error {
	if filename == "" {
		return errors.New("filename is required")
	}
	if !allowedTypes[contentType] {
		return errors.New("content type is not allowed")
	}
	if byteSize <= 0 || byteSize > MaxUploadBytes {
		return errors.New("file size is invalid")
	}
	return nil
}

func ValidAssetStatus(status string) bool {
	switch status {
	case StatusActive, StatusArchived, StatusDeleted:
		return true
	default:
		return false
	}
}
