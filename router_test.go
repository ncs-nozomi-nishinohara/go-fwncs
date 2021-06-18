package fwncs_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/constant"
	"github.com/n-creativesystem/go-fwncs/tests"
	"github.com/stretchr/testify/assert"
)

func getClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{}
	*client = *http.DefaultClient
	client.Transport = tr
	return client
}

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
				assert.Equal(t, constant.JSON.String(), rw.Header().Get("Content-Type"))
				req = httptest.NewRequest(http.MethodPost, "/", nil)
				rw = httptest.NewRecorder()
				route.ServeHTTP(rw, req)
				defer rw.Result().Body.Close()
				assert.Equal(t, 405, rw.Result().StatusCode)
				assert.Equal(t, constant.JSON.String(), rw.Header().Get("Content-Type"))
			},
		},
		{
			Name: "use add route",
			Fn: func(t *testing.T) {
				signature := ""
				route := fwncs.Default()
				route.Use(func(c fwncs.Context) {
					signature += "B"
					c.Next()
					signature += "C"
				})
				route.GET("", func(c fwncs.Context) {
					signature += "A"
					c.JSON(200, map[string]string{"test": "OK"})
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
			Name: "panic middleware test",
			Fn: func(t *testing.T) {
				route := fwncs.Default()
				route.Use(func(c fwncs.Context) {
					fn := func() {
						panic("panic test")
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
					assert.Equal(t, "{\"code\":500,\"status\":\"error\",\"message\":\"panic test\"}\n", string(buf))
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
					err := c.ReadJsonBody(&b)
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

func TestRun(t *testing.T) {
	router := fwncs.Default()
	router.GET("", func(c fwncs.Context) {
		c.JSON(http.StatusOK, "OK")
	})
	go func() {
		assert.NoError(t, router.Run(8080))
	}()
	time.Sleep(3 * time.Second)
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
	client := http.DefaultClient
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRunTLS(t *testing.T) {
	router := fwncs.Default()
	router.GET("", func(c fwncs.Context) {
		c.JSON(http.StatusOK, "OK")
	})
	go func() {
		assert.NoError(t, router.RunTLS(8443, "tests/server.crt", "tests/server.key"))
	}()
	time.Sleep(3 * time.Second)
	req, _ := http.NewRequest(http.MethodGet, "https://localhost:8443", nil)
	client := http.DefaultClient
	client.Timeout = 5 * time.Second
	_, err := client.Do(req)
	assert.Error(t, err)

	client = getClient()
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestMiddleware(t *testing.T) {
	test := ""
	router := fwncs.Default()
	router.Use(func(c fwncs.Context) {
		test += "a"
	})
	router.GET("/", func(c fwncs.Context) {
		test += "b"
	})
	router.Use(func(c fwncs.Context) {
		test += "c"
	})
	router.GET("/test", func(c fwncs.Context) {
		test += "d"
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Result().Body.Close()
	assert.Equal(t, "ab", test)
	test = ""
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Result().Body.Close()
	assert.Equal(t, "acd", test)
}
