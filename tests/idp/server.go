package idp

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/form3tech-oss/jwt-go"
	"github.com/n-creativesystem/go-fwncs"
	"gopkg.in/square/go-jose.v2"
)

const (
	AuthorizeEndpoint           = "/authorize"
	TokenEndpoint               = "/oauth/token"
	WellKnownEndpoint           = "/.well-known"
	OpenIDConfigurationEndpoint = "/openid-configuration"
	JWKSEndpoint                = "/jwks.json"
	LoginEndpoint               = "/u/login"
)

type provider struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	JWKSEndpoint                     string   `json:"jwks_uri"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint"`
	IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

type token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IdToken      string `json:"id_token"`
}

type IdentityProvider struct {
	Issuer     string
	PrivateKey *rsa.PrivateKey
	codes      map[string]struct{}
	handler    http.Handler
	listen     net.Listener
}

func (idp *IdentityProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idp.handler.ServeHTTP(w, r)
}

func (idp *IdentityProvider) Run() {
	http.Serve(idp.listen, idp)
}

func NewIdpServer() *IdentityProvider {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	issuer := "http://127.0.0.1:9999"
	idp := &IdentityProvider{
		Issuer:     issuer,
		PrivateKey: privateKey,
		codes:      map[string]struct{}{},
	}
	l, err := net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	idp.listen = l

	router := fwncs.Default()
	wellKnown := router.Group(WellKnownEndpoint)
	wellKnown.GET(OpenIDConfigurationEndpoint, func(c fwncs.Context) {
		p := &provider{
			Issuer:                           issuer,
			AuthorizationEndpoint:            issuer + AuthorizeEndpoint,
			TokenEndpoint:                    issuer + TokenEndpoint,
			JWKSEndpoint:                     issuer + WellKnownEndpoint + JWKSEndpoint,
			IdTokenSigningAlgValuesSupported: []string{jwt.SigningMethodRS256.Alg()},
		}
		c.JSON(http.StatusOK, p)
	})
	wellKnown.GET(JWKSEndpoint, func(c fwncs.Context) {
		jwks := &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: idp.PrivateKey.Public(), KeyID: "idp"},
			},
		}
		c.JSON(http.StatusOK, jwks)
	})
	router.GET(AuthorizeEndpoint, func(c fwncs.Context) {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/%s?%s", idp.Issuer, "u/login", c.Request().URL.Query().Encode()))
	})
	router.POST(LoginEndpoint, idp.handlerLogin)
	router.GET(TokenEndpoint, idp.handleToken)
	idp.handler = router

	return idp
}

func (i *IdentityProvider) handlerLogin(c fwncs.Context) {
	permissions := []string{}
	cs := &ClaimSet{
		Iss: i.Issuer,
		Aud: "local-provider",
		PrivateClaims: map[string]interface{}{
			"email": "local-provider@n-creativesystem.dev",
		},
	}
	type user struct {
		UserId   string `json:"user_id"`
		Password string `json:"password"`
		Exp      int64  `json:"exp"`
		Iat      int64  `json:"iat"`
	}
	var u user
	c.ReadJsonBody(&u)
	if u.UserId == "admin" {
		permissions = append(permissions, "read:query")
	}
	if u.Exp != 0 {
		cs.Exp = u.Exp
	}
	if u.Iat != 0 {
		cs.Iat = u.Iat
	}
	cs.PrivateClaims["permission"] = permissions
	idToken, err := Encode(&Header{Algorithm: jwt.SigningMethodRS256.Alg(), KeyID: "idp"}, cs, i.PrivateKey)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	token := &token{
		AccessToken:  "accesstoken",
		RefreshToken: "refreshtoken",
		IdToken:      idToken,
	}
	c.JSON(http.StatusOK, token)
}

func (i *IdentityProvider) handleToken(c fwncs.Context) {
	r := c.Request()
	if err := r.ParseForm(); err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	gotCode := r.FormValue("code")
	if _, ok := i.codes[gotCode]; !ok {
		log.Print("Unknown code")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	cs := &ClaimSet{
		Iss:           i.Issuer,
		Aud:           "local-provider",
		PrivateClaims: map[string]interface{}{"email": "local-provider@n-creativesystem.dev"},
	}
	idToken, err := Encode(&Header{Algorithm: "RS256", KeyID: "idp"}, cs, i.PrivateKey)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	token := &token{
		AccessToken:  "accesstoken",
		RefreshToken: "refreshtoken",
		IdToken:      idToken,
	}
	c.JSON(http.StatusOK, token)
}
