package generate

import (
	"crypto/rand"
	"strings"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// SecureStringFromAlphabet generates a n-length string
func SecureStringFromAlphabet(n int, alphabet string) (string, error) {
	arune := []rune(alphabet)

	bytes, err := generateRandomBytes(n)
	if err != nil {
		return "", err
	}

	var out strings.Builder

	for _, b := range bytes {
		out.WriteRune(arune[int(b)%len(arune)])
	}

	return out.String(), nil
}
