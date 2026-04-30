package articles

import "regexp"

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}
