package articles

import "testing"

func TestValidateSlug(t *testing.T) {
	valid := []string{
		"aB3dE9",
		"000001",
		"Z9yX8w",
	}
	for _, slug := range valid {
		if !ValidSlug(slug) {
			t.Fatalf("ValidSlug(%q) = false, want true", slug)
		}
	}

	invalid := []string{
		"",
		"go-concurrency",
		"abcde",
		"abcdefg",
		"abc_def",
		"has space",
		"中文",
	}
	for _, slug := range invalid {
		if ValidSlug(slug) {
			t.Fatalf("ValidSlug(%q) = true, want false", slug)
		}
	}
}

func TestGenerateShortSlugReturnsSixBase62Characters(t *testing.T) {
	for range 100 {
		slug, err := GenerateShortSlug()
		if err != nil {
			t.Fatalf("GenerateShortSlug() error = %v", err)
		}
		if !ValidSlug(slug) {
			t.Fatalf("GenerateShortSlug() = %q, want six Base62 characters", slug)
		}
	}
}

func TestStableShortSlugFromStringIsDeterministic(t *testing.T) {
	first := StableShortSlugFromString("redis-sentinel-with-docker")
	second := StableShortSlugFromString("redis-sentinel-with-docker")
	other := StableShortSlugFromString("postgres-search")

	if first != second {
		t.Fatalf("StableShortSlugFromString returned %q then %q", first, second)
	}
	if first == other {
		t.Fatalf("StableShortSlugFromString collision for test inputs: %q", first)
	}
	if !ValidSlug(first) || !ValidSlug(other) {
		t.Fatalf("stable slugs should be six Base62 characters: %q %q", first, other)
	}
}
