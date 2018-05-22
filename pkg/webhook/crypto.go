package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// SHA1HMAC computes the GitHub SHA1 HMAC.
func SHA1HMAC(salt, message []byte) string {
	// GitHub creates a SHA1 HMAC, where the key is the GitHub secret and the
	// message is the JSON body.
	digest := hmac.New(sha1.New, salt)
	digest.Write(message)
	sum := digest.Sum(nil)
	return fmt.Sprintf("sha1=%x", sum)
}

func JWT(appID string, keyPEM []byte) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claim := &jwt.StandardClaims{
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
		Issuer:    appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claim)
	return token.SignedString(key)
}
