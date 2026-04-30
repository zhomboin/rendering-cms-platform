package articles

import "testing"

func TestValidateSlug(t *testing.T) {
	valid := []string{
		"go-concurrency",
		"react19",
		"building-modern-ui",
	}
	for _, slug := range valid {
		if !ValidSlug(slug) {
			t.Fatalf("ValidSlug(%q) = false, want true", slug)
		}
	}

	invalid := []string{
		"",
		"Go-Concurrency",
		"-leading",
		"trailing-",
		"double--dash",
		"has space",
		"中文",
	}
	for _, slug := range invalid {
		if ValidSlug(slug) {
			t.Fatalf("ValidSlug(%q) = true, want false", slug)
		}
	}
}
