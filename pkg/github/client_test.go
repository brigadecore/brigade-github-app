package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	baseURL   = "http://example.com/base/api/v3/"
	uploadURL = "http://example.com/upload/api/v3/api/uploads/"
	testToken = "totallyFake"
)

func TestNewClientFromBearerToken(t *testing.T) {
	ghc, err := NewClientFromBearerToken(baseURL, uploadURL, testToken)
	require.NoError(t, err)
	require.Equal(t, baseURL, ghc.BaseURL.String())
	require.Equal(t, uploadURL, ghc.UploadURL.String())
}

func TestNewClientFromInstallationToken(t *testing.T) {
	ghc, err := NewClientFromInstallationToken(baseURL, uploadURL, testToken)
	require.NoError(t, err)
	require.Equal(t, baseURL, ghc.BaseURL.String())
	require.Equal(t, uploadURL, ghc.UploadURL.String())
}
