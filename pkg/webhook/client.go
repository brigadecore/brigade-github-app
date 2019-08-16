package webhook

import (
	"context"
	"strconv"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"

	"github.com/brigadecore/brigade/pkg/brigade"
)

// ghClient gets a new GitHub client object.
//
// It authenticates with an OAUTH2 token.
//
// If an enterpriseHost base URL is provided, this will attempt to connect to
// that instead of the hosted GitHub API server.
func ghClient(gh brigade.Github) (*github.Client, error) {
	t := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gh.Token})
	c := context.Background()
	tc := oauth2.NewClient(c, t)
	if gh.BaseURL != "" {
		return github.NewEnterpriseClient(gh.BaseURL, gh.UploadURL, tc)
	}
	return github.NewClient(tc), nil
}

func (s *githubHook) installationToken(appID, installationID int, cfg brigade.Github) (string, time.Time, error) {
	aidStr := strconv.Itoa(appID)
	// We need to perform auth here, and then inject the token into the
	// body so that the app can use it.
	tok, err := JWT(aidStr, s.key)
	if err != nil {
		return "", time.Time{}, err
	}

	ghc, err := ghClient(brigade.Github{
		Token:     tok,
		BaseURL:   cfg.BaseURL,
		UploadURL: cfg.UploadURL,
	})

	if err != nil {
		return "", time.Time{}, err
	}

	ctx := context.Background()
	itok, _, err := ghc.Apps.CreateInstallationToken(ctx, int64(installationID))
	if err != nil {
		return "", time.Time{}, err
	}
	return itok.GetToken(), itok.GetExpiresAt(), nil
}

// InstallationTokenClient uses an installation token to authenticate to the Github API.
func InstallationTokenClient(instToken, baseURL, uploadURL string) (*github.Client, error) {
	// For installation tokens, Github uses a different token type ("token" instead of "bearer")
	t := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: instToken, TokenType: "token"})
	c := context.Background()
	tc := oauth2.NewClient(c, t)
	if baseURL != "" {
		return github.NewEnterpriseClient(baseURL, uploadURL, tc)
	}
	return github.NewClient(tc), nil
}
