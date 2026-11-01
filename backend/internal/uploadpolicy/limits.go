package uploadpolicy

const (
	ImageBytes        = 50 * 1024 * 1024
	FileBytes         = 20 * 1024 * 1024
	PhotoRequestBytes = 51 * 1024 * 1024
)

func MaxBytes(contentType string) int {
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return ImageBytes
	default:
		return FileBytes
	}
}
