package articles

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"regexp"
)

const (
	shortSlugLength = 6
	shortSlugSpace  = 56800235584 // 62^6
)

const shortSlugAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var slugPattern = regexp.MustCompile(`^[0-9A-Za-z]{6}$`)
var articleNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

func ValidArticleName(articleName string) bool {
	return articleNamePattern.MatchString(articleName)
}

func GenerateShortSlug() (string, error) {
	result := make([]byte, shortSlugLength)
	max := big.NewInt(int64(len(shortSlugAlphabet)))
	for index := range result {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[index] = shortSlugAlphabet[value.Int64()]
	}
	return string(result), nil
}

func StableShortSlugFromString(value string) string {
	sum := md5.Sum([]byte(value))
	number := binary.BigEndian.Uint64(append([]byte{0, 0}, sum[:6]...)) % shortSlugSpace
	return encodeBase62Fixed(number)
}

func encodeBase62Fixed(value uint64) string {
	result := make([]byte, shortSlugLength)
	for index := shortSlugLength - 1; index >= 0; index-- {
		result[index] = shortSlugAlphabet[value%uint64(len(shortSlugAlphabet))]
		value = value / uint64(len(shortSlugAlphabet))
	}
	return string(result)
}
