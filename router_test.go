package fwncs_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/tests"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRouter(t *testing.T) {
	tt := tests.TestFrames{
		{
			Name: "normal route",
			Fn: func(t *testing.T) {
				route := fwncs.Default()
				route.GET("", func(c fwncs.Context) {
					c.JSON(200, map[string]string{"test": "OK"})
				})
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rw := httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				if assert.Equal(t, 200, rw.Result().StatusCode) {
					defer rw.Result().Body.Close()
					buf, _ := io.ReadAll(rw.Result().Body)
					assert.Equal(t, "{\"test\":\"OK\"}\n", string(buf))
				}
				req = httptest.NewRequest(http.MethodGet, "/not-found", nil)
				rw = httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, 404, rw.Result().StatusCode)
				assert.Equal(t, "application/json", rw.Header().Get("Content-Type"))
				req = httptest.NewRequest(http.MethodPost, "/", nil)
				rw = httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, 405, rw.Result().StatusCode)
				assert.Equal(t, "application/json", rw.Header().Get("Content-Type"))
			},
		},
		{
			Name: "use add route",
			Fn: func(t *testing.T) {
				signature := ""
				route := fwncs.Default()
				route.GET("", func(c fwncs.Context) {
					signature += "A"
					c.JSON(200, map[string]string{"test": "OK"})
				})
				route.Use(func(c fwncs.Context) {
					signature += "B"
					c.Next()
					signature += "C"
				})
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rw := httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, "BAC", signature)
			},
		},
		{
			Name: "group router",
			Fn: func(t *testing.T) {
				signature := ""
				route := fwncs.Default()
				route.Use(func(c fwncs.Context) {
					signature += "A"
					c.Next()
					signature += "B"
				})
				v1 := route.Group("v1")
				{
					v1.Use(func(c fwncs.Context) {
						signature += "C"
						c.Next()
						signature += "D"
					}, func(c fwncs.Context) {
						signature += "E"
						c.Next()
						signature += "G"
					})
					v1.GET("", func(c fwncs.Context) {
						signature += "H"
						c.JSON(200, map[string]string{"V1": "OK"})
					})
				}
				v2 := route.Group("v2")
				{
					v2.Use(func(c fwncs.Context) {
						signature += "1"
						c.Next()
						signature += "2"
					}, func(c fwncs.Context) {
						signature += "3"
						c.Next()
						signature += "4"
					})
					v2.GET("", func(c fwncs.Context) {
						signature += "5"
						c.JSON(200, map[string]string{"V2": "OK"})
					})
				}
				req := httptest.NewRequest(http.MethodGet, "/v1", nil)
				rw := httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				assert.Equal(t, "ACEHGDB", signature)
				buf, _ := io.ReadAll(rw.Result().Body)
				assert.Equal(t, "{\"V1\":\"OK\"}\n", string(buf))
				rw.Result().Body.Close()
				signature = ""
				req = httptest.NewRequest(http.MethodGet, "/v2", nil)
				rw = httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				buf, _ = io.ReadAll(rw.Result().Body)
				assert.Equal(t, "{\"V2\":\"OK\"}\n", string(buf))
				rw.Result().Body.Close()
				assert.Equal(t, "A13542B", signature)
			},
		},
		{
			Name: "panic handler",
			Fn: func(t *testing.T) {
				route := fwncs.Default()
				route.Use(func(c fwncs.Context) {
					fn := func() {
						panic("test")
					}
					fn()
				})
				route.GET("", func(c fwncs.Context) {
					c.JSON(http.StatusOK, fwncs.NewDefaultResponseBody(http.StatusOK, http.StatusText(http.StatusOK)))
				})
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rw := httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				if assert.Equal(t, http.StatusInternalServerError, rw.Result().StatusCode) {
					defer rw.Result().Body.Close()
					buf, _ := io.ReadAll(rw.Result().Body)
					assert.Equal(t, "{\"status\":\"500\",\"message\":\"test\"}\n", string(buf))
				}
			},
		},
		{
			Name: "abort handler",
			Fn: func(t *testing.T) {
				type body struct {
					UserId string `json:"user_id"`
				}
				route := fwncs.Default()
				route.Use(func(c fwncs.Context) {
					b := body{}
					err := c.JSONBody(&b)
					assert.NoError(t, err)
					if b.UserId != "admin" {
						c.AbortWithStatus(http.StatusUnauthorized)
					} else {
						c.Next()
					}
				})
				route.POST("", func(c fwncs.Context) {
					c.JSON(http.StatusOK, fwncs.NewDefaultResponseBody(http.StatusOK, http.StatusText(http.StatusOK)))
				})
				b := body{
					UserId: "user",
				}
				buf, _ := json.Marshal(&b)
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(buf))
				rw := httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				assert.Equal(t, http.StatusUnauthorized, rw.Result().StatusCode)
				defer rw.Result().Body.Close()
				b = body{
					UserId: "admin",
				}
				buf, _ = json.Marshal(&b)
				req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(buf))
				rw = httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				assert.Equal(t, http.StatusOK, rw.Result().StatusCode)
			},
		},
	}
	tt.Run(t)
}
