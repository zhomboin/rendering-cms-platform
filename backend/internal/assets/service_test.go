package assets

import "testing"

func TestValidateUpload(t *testing.T) {
	if err := ValidateUpload("a.pdf", "application/pdf", 1024); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpload("a.exe", "application/octet-stream", 1024); err == nil {
		t.Fatal("expected invalid content type")
	}
	if err := ValidateUpload("big.pdf", "application/pdf", 20*1024*1024+1); err == nil {
		t.Fatal("expected size error")
	}
	if err := ValidateUpload("", "application/pdf", 1024); err == nil {
		t.Fatal("expected filename error")
	}
}

func TestValidAssetStatus(t *testing.T) {
	for _, status := range []string{StatusActive, StatusArchived, StatusDeleted} {
		if !ValidAssetStatus(status) {
			t.Fatalf("ValidAssetStatus(%q) = false, want true", status)
		}
	}
	if ValidAssetStatus("pending") {
		t.Fatal("pending should not be a valid asset status")
	}
}

func TestValidateUploadTypeSizeBoundaries(t *testing.T) {
	for _, contentType := range []string{"image/png", "image/jpeg", "image/webp", "application/pdf", "text/plain", "application/zip"} {
		limit := 20 * 1024 * 1024
		if contentType[:6] == "image/" {
			limit = 50 * 1024 * 1024
		}
		for _, size := range []int{0, limit - 1, limit, limit + 1} {
			err := ValidateUpload("file", contentType, size)
			valid := size > 0 && size <= limit
			if (err == nil) != valid {
				t.Errorf("type=%s size=%d err=%v, valid=%v", contentType, size, err, valid)
			}
		}
	}
}
