module github.com/brigadecore/brigade-github-app

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.18.2

require (
	github.com/brigadecore/brigade v1.4.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/go-github/v18 v18.2.0
	github.com/stretchr/testify v1.4.0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/gin-gonic/gin.v1 v1.1.5-0.20170702092826-d459835d2b07
	k8s.io/api v0.18.2
)
