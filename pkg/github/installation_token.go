package github

import (
	"context"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-github/v32/github"
)

// GetInstallationToken returns an installation token and its expiry time for
// the given baseURL, uploadURL, appID, and installationID. It uses the provided
// ASCII-armored x509 certificate key to sign a JSON web token that is then
// exchanged for the installation token. If baseURL is the empty string, the
// client used in this process will be one for github.com. Otherwise, the client
// will be one for GitHub Enterprise.
func GetInstallationToken(
	baseURL string,
	uploadURL string,
	appID int64,
	installationID int64,
	keyPEM []byte,
) (string, time.Time, error) {
	// Construct a JSON web token to use as the bearer token to create a new
	// client that we can use to, in turn, create the installation token.
	jsonWebToken, err := getSignedJSONWebToken(appID, keyPEM)
	if err != nil {
		return "", time.Time{}, err
	}
	githubClient, err := NewClientFromBearerToken(baseURL, uploadURL, jsonWebToken)
	if err != nil {
		return "", time.Time{}, err
	}
	installationToken, _, err := githubClient.Apps.CreateInstallationToken(
		context.Background(),
		installationID,
		&github.InstallationTokenOptions{},
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return installationToken.GetToken(), installationToken.GetExpiresAt(), nil
}

// getSignedJSONWebToken constructs, signs, and returns a JSON web token.
func getSignedJSONWebToken(appID int64, keyPEM []byte) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		return "", err
	}
	now := time.Now()
	return jwt.NewWithClaims(
		jwt.SigningMethodRS256,
		jwt.StandardClaims{
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(5 * time.Minute).Unix(),
			Issuer:    strconv.FormatInt(appID, 10),
		},
	).SignedString(key)
}
