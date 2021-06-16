package fwncs_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/tests"
	"github.com/n-creativesystem/go-fwncs/tests/idp"
	"github.com/stretchr/testify/assert"
)

func TestAuthorization(t *testing.T) {
	var idpServer = idp.NewIdpServer()
	go idpServer.Run()
	router := fwncs.Default()
	router.Use(fwncs.Auth(fwncs.AuthOption{
		Issuer:   "http://127.0.0.1:9999",
		ClientID: "local-client",
		KeyFunc: func(ctx context.Context, jwksURL, kid string) (interface{}, error) {
			req, _ := http.NewRequest(http.MethodGet, jwksURL, nil)
			client := http.DefaultClient
			client.Transport = fwncs.RequestIDTransport(ctx, nil)
			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			buf, _ := io.ReadAll(resp.Body)
			var body map[string]interface{}
			err = json.Unmarshal(buf, &body)
			if err != nil {
				return nil, err
			}
			for _, bodyKey := range body["keys"].([]interface{}) {
				key := bodyKey.(map[string]interface{})
				_kid := key["kid"].(string)
				if kid == _kid {
					rsaKey := new(rsa.PublicKey)
					number, _ := base64.RawURLEncoding.DecodeString(key["n"].(string))
					rsaKey.N = new(big.Int).SetBytes(number)
					rsaKey.E = 65537
					return rsaKey, nil
				}
			}
			return nil, fmt.Errorf("Not found kid: %s", kid)
		},
	}))
	router.GET("", func(c fwncs.Context) {
		c.JSON(http.StatusOK, fwncs.NewDefaultResponseBody(http.StatusOK, "OK"))
	})
	router.GET("/permission", fwncs.Permission("permission", "read:query"), func(c fwncs.Context) {
		c.JSON(http.StatusOK, fwncs.NewDefaultResponseBody(http.StatusOK, "Permission is ok"))
	})
	type user struct {
		UserId   string `json:"user_id"`
		Password string `json:"password"`
		Exp      int64  `json:"exp"`
		Iat      int64  `json:"iat"`
	}
	type token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IdToken      string `json:"id_token"`
	}
	getIdToken := func(u *user) token {
		buf, _ := json.Marshal(u)
		req := httptest.NewRequest(http.MethodPost, idp.LoginEndpoint, bytes.NewBuffer(buf))
		rw := httptest.NewRecorder()
		idpServer.ServeHTTP(rw, req)
		defer rw.Result().Body.Close()
		buf, _ = io.ReadAll(rw.Result().Body)
		var _token token
		json.Unmarshal(buf, &_token)
		return _token
	}
	tt := tests.TestFrames{
		{
			Name: "No authorization header",
			Fn: func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rw := httptest.NewRecorder()
				router.ServeHTTP(rw, req)
				assert.Equal(t, rw.Result().StatusCode, http.StatusUnauthorized)
			},
		},
		{
			Name: "Expired token",
			Fn: func(t *testing.T) {
				now := time.Now()
				u := &user{
					UserId:   "admin",
					Password: "password",
					Exp:      now.Add(1 * time.Microsecond).UnixNano(),
					Iat:      now.UnixNano(),
				}
				_token := getIdToken(u)
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", _token.IdToken))
				rw := httptest.NewRecorder()
				router.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, rw.Result().StatusCode, http.StatusUnauthorized)
				assert.Equal(t, "Bearer error=\"invalid_request\"", rw.Header().Get("WWW-Authenticate"))
				buf, _ := io.ReadAll(rw.Result().Body)
				assert.Equal(t, "{\"status\":401,\"message\":\"error\",\"message_describe\":\"Token used before issued\"}\n", string(buf))
			},
		},
		{
			Name: "authorization ok",
			Fn: func(t *testing.T) {
				u := &user{
					UserId:   "admin",
					Password: "pass",
				}
				_token := getIdToken(u)
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", _token.IdToken))
				rw := httptest.NewRecorder()
				router.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				buf, _ := io.ReadAll(rw.Result().Body)
				assert.Equal(t, "{\"code\":200,\"status\":\"success\",\"message\":\"OK\"}\n", string(buf))
			},
		},
		{
			Name: "permission ok",
			Fn: func(t *testing.T) {
				u := &user{
					UserId:   "admin",
					Password: "pass",
				}
				_token := getIdToken(u)
				req := httptest.NewRequest(http.MethodGet, "/permission", nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", _token.IdToken))
				rw := httptest.NewRecorder()
				router.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				buf, _ := io.ReadAll(rw.Result().Body)
				assert.Equal(t, "{\"code\":200,\"status\":\"success\",\"message\":\"Permission is ok\"}\n", string(buf))
			},
		},
		{
			Name: "permission error",
			Fn: func(t *testing.T) {
				u := &user{
					UserId:   "user",
					Password: "pass",
				}
				_token := getIdToken(u)
				req := httptest.NewRequest(http.MethodGet, "/permission", nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", _token.IdToken))
				rw := httptest.NewRecorder()
				router.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, rw.Result().StatusCode, http.StatusForbidden)
				assert.Equal(t, "Bearer error=\"insufficient_scope\"", rw.Header().Get("WWW-Authenticate"))
			},
		},
	}
	tt.Run(t)
}
