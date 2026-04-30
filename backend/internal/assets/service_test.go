package assets

import "testing"

func TestValidateUpload(t *testing.T) {
	if err := ValidateUpload("a.pdf", "application/pdf", 1024); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpload("a.exe", "application/octet-stream", 1024); err == nil {
		t.Fatal("expected invalid content type")
	}
	if err := ValidateUpload("big.pdf", "application/pdf", MaxUploadBytes+1); err == nil {
		t.Fatal("expected size error")
	}
	if err := ValidateUpload("", "application/pdf", 1024); err == nil {
		t.Fatal("expected filename error")
	}
}
