package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// NewClientFromBearerToken returns a new github.Client for the given baseURL,
// uploadURL and bearer token. If baseURL is the empty string, the client will
// be for github.com. Otherwise, the client will be one for GitHub Enterprise.
func NewClientFromBearerToken(
	baseURL string,
	uploadURL string,
	token string,
) (*github.Client, error) {
	return newClient(
		baseURL,
		uploadURL,
		oauth2.StaticTokenSource(
			&oauth2.Token{
				AccessToken: token,
			},
		),
	)
}

// NewClientFromInstallationToken returns a new github.Client for the given
// baseURL, uploadURL and installation token. If baseURL is the empty string,
// the client will be for github.com. Otherwise, the client will be one for
// GitHub Enterprise.
func NewClientFromInstallationToken(
	baseURL string,
	uploadURL string,
	token string,
) (*github.Client, error) {
	return newClient(
		baseURL,
		uploadURL,
		oauth2.StaticTokenSource(
			&oauth2.Token{
				TokenType:   "token", // This type indicates an installation token
				AccessToken: token,
			},
		),
	)
}

// NewClientFromKeyPEM returns a new github.Client for the given baseURL,
// uploadURL, appID, and installationID. It uses the provided ASCII-armored x509
// certificate key to sign a JSON web token that is then exchanged for an
// installation token that will ultimately be used by the returned client. If
// baseURL is the empty string, the client will be for github.com. Otherwise,
// the client will be one for GitHub Enterprise.
func NewClientFromKeyPEM(
	baseURL string,
	uploadURL string,
	appID int64,
	installationID int64,
	keyPEM []byte,
) (*github.Client, error) {
	installationToken, _, err := GetInstallationToken(
		baseURL,
		uploadURL,
		appID,
		installationID,
		keyPEM,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to negotiate an installation token: %s", err)
	}
	return newClient(
		baseURL,
		uploadURL,
		oauth2.StaticTokenSource(
			&oauth2.Token{
				TokenType:   "token", // This type indicates an installation token
				AccessToken: installationToken,
			},
		),
	)
}

// newClient returns a new github.Client for the given baseURL, uploadURL and
// oauth2.TokenSource. If baseURL is the empty string, the client will be for
// github.com. Otherwise, the client will be one for GitHub Enterprise.
func newClient(
	baseURL,
	uploadURL string,
	tokenSource oauth2.TokenSource,
) (*github.Client, error) {
	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	if baseURL == "" {
		return github.NewClient(httpClient), nil
	}
	return github.NewEnterpriseClient(baseURL, uploadURL, httpClient)
}
