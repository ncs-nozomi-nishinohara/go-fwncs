package fwncs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/form3tech-oss/jwt-go"
)

const (
	AuthKey = "auth_key"
)

var (
	ErrEmptyAuthorization  = errors.New("token_required")
	ErrInvalidTokenRequest = errors.New("invalid_request")
	ErrInvalidToken        = errors.New("invalid_token")
)

type TokenExtractorFunc func(r *http.Request) (string, error)

type KeyFunc func(ctx context.Context, jwksURL, kid string) (interface{}, error)

func FromHeader(schema string) func(r *http.Request) (string, error) {
	return func(r *http.Request) (string, error) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return "", ErrEmptyAuthorization
		}
		authHeaders := strings.Fields(authHeader)
		if len(authHeaders) != 2 {
			return "", ErrInvalidTokenRequest
		}
		if strings.ToLower(authHeaders[0]) != schema {
			return "", ErrInvalidTokenRequest
		}
		return authHeaders[1], nil
	}
}

func FromParameter(parameterName string) TokenExtractorFunc {
	return func(r *http.Request) (string, error) {
		return r.URL.Query().Get(parameterName), nil
	}
}

type providerJSON struct {
	Issuer      string   `json:"issuer"`
	AuthURL     string   `json:"authorization_endpoint"`
	TokenURL    string   `json:"token_endpoint"`
	JWKSURL     string   `json:"jwks_uri"`
	UserInfoURL string   `json:"userinfo_endpoint"`
	Algorithms  []string `json:"id_token_signing_alg_values_supported"`
}

var supportedAlgorithms = map[string]bool{
	oidc.RS256: true,
	oidc.RS384: true,
	oidc.RS512: true,
	oidc.ES256: true,
	oidc.ES384: true,
	oidc.ES512: true,
	oidc.PS256: true,
	oidc.PS384: true,
	oidc.PS512: true,
}

type provider struct {
	issuer      string
	authURL     string
	tokenURL    string
	userInfoURL string
	jwksURL     string
	algorithms  []string
}

func newProvider(issuer string) (*provider, error) {
	wellKnown := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequest("GET", wellKnown, nil)
	if err != nil {
		return nil, err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	// resp, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, body)
	}

	var p providerJSON
	err = json.Unmarshal(body, &p)
	if err != nil {
		return nil, fmt.Errorf("oidc: failed to decode provider discovery object: %v", err)
	}

	if p.Issuer != issuer {
		return nil, fmt.Errorf("oidc: issuer did not match the issuer returned by provider, expected %q got %q", issuer, p.Issuer)
	}
	var algs []string
	for _, a := range p.Algorithms {
		if supportedAlgorithms[a] {
			algs = append(algs, a)
		}
	}
	return &provider{
		issuer:      p.Issuer,
		authURL:     p.AuthURL,
		tokenURL:    p.TokenURL,
		userInfoURL: p.UserInfoURL,
		jwksURL:     p.JWKSURL,
		algorithms:  algs,
	}, nil
}

type AuthOption struct {
	Issuer        string
	Audiences     []string
	ClientID      string
	EnableOptions bool
	Extractor     TokenExtractorFunc
	KeyFunc       KeyFunc
	Schema        string
}

func Auth(opt AuthOption) HandlerFunc {
	schema := "bearer"
	if opt.Schema != "" {
		schema = opt.Schema
	}
	if opt.Extractor == nil {
		opt.Extractor = FromHeader(schema)
	}
	provider, err := newProvider(opt.Issuer)
	if err != nil {
		panic(err)
	}
	if opt.KeyFunc == nil {
		panic(errors.New("KeyFunc is nil"))
	}
	return func(c Context) {
		if !opt.EnableOptions {
			if c.Request().Method == "OPTIONS" {
				c.Next()
				return
			}
		}
		authToken, err := opt.Extractor(c.Request())
		if err != nil {
			switch err {
			case ErrEmptyAuthorization:
				c.SetHeader("WWW-Authenticate", "Bearer realm=\"token_required\"")
			case ErrInvalidTokenRequest:
				c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_request\"")
			default:
				c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_request\"")
			}
			c.AbortWithStatusAndErrorMessage(http.StatusUnauthorized, err)
			return
		}
		token, err := jwt.Parse(authToken, func(t *jwt.Token) (interface{}, error) {
			kid, ok := t.Header["kid"].(string)
			if !ok {
				return nil, errors.New("Invalid kid")
			}
			return opt.KeyFunc(c.GetContext(), provider.jwksURL, kid)
		})
		if err != nil {
			c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_request\"")
			c.AbortWithStatusAndErrorMessage(http.StatusUnauthorized, err)
			return
		}
		if !token.Valid {
			c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_token\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		mpClaim := token.Claims.(jwt.MapClaims)
		if err := mpClaim.Valid(); err != nil {
			c.Logger().Error(err)
			c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_token\"")
			c.AbortWithStatusAndErrorMessage(http.StatusUnauthorized, err)
			return
		}
		iss, ok := mpClaim["iss"].(string)
		if !ok {
			c.Logger().Error("Invalid issuer")
			c.AbortWithStatus(http.StatusUnauthorized)
		}
		if !strings.Contains(iss, opt.Issuer) {
			c.Logger().Error(fmt.Sprintf("Invalid issuer: %s", opt.Issuer))
			c.AbortWithStatus(http.StatusUnauthorized)
		}
		if len(opt.Audiences) > 0 {
			noAudience := true
			audience, ok := mpClaim["aud"].(string)
			if ok {
				for _, aud := range opt.Audiences {
					if strings.Contains(audience, aud) {
						noAudience = false
						break
					}
				}
			}
			if noAudience {
				c.AbortWithStatus(http.StatusUnauthorized)
			}
		}
		if !c.IsAbort() {
			c.Set(AuthKey, token)
			c.Next()
		} else {
			c.SetHeader("WWW-Authenticate", "Bearer error=\"invalid_token\"")
		}
	}
}
