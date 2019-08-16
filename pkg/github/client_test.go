package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	baseURL   = "http://example.com/base/"
	uploadURL = "http://example.com/upload/"
	testToken = "totallyFake"
)

func TestNewClientFromBearerToken(t *testing.T) {
	ghc, err := NewClientFromBearerToken(baseURL, uploadURL, testToken)
	require.NoError(t, err)
	require.Equal(t, baseURL, ghc.BaseURL.String())
	require.Equal(t, baseURL, ghc.BaseURL.String())
}

func TestNewClientFromInstallationToken(t *testing.T) {
	ghc, err := NewClientFromInstallationToken(baseURL, uploadURL, testToken)
	require.NoError(t, err)
	require.Equal(t, baseURL, ghc.BaseURL.String())
	require.Equal(t, baseURL, ghc.BaseURL.String())
}
