package tester

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/sessions"
	"github.com/stretchr/testify/assert"
)

type storeFactory func(*testing.T) sessions.Store

const sessionName = "session"

const ok = "ok"

func GetSet(t *testing.T, newStore storeFactory) {
	r := fwncs.Default()
	r.Use(sessions.Sessions("session", newStore(t)))
	r.GET("/set", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Set("key", ok)
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/get", func(c fwncs.Context) {
		session := sessions.Default(c)
		assert.Equal(t, ok, session.Get("key"))
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})
	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/get", nil)
	req2.Header.Set("Cookie", res1.Header().Get("Set-Cookie"))
	r.ServeHTTP(res2, req2)
}

func DeleteKey(t *testing.T, newStore storeFactory) {
	r := fwncs.Default()
	r.Use(sessions.Sessions(sessionName, newStore(t)))

	r.GET("/set", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Set("key", ok)
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/delete", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Delete("key")
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/get", func(c fwncs.Context) {
		session := sessions.Default(c)
		if session.Get("key") != nil {
			t.Error("Session deleting failed")
		}
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/delete", nil)
	req2.Header.Set("Cookie", res1.Header().Get("Set-Cookie"))
	r.ServeHTTP(res2, req2)

	res3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/get", nil)
	req3.Header.Set("Cookie", res2.Header().Get("Set-Cookie"))
	r.ServeHTTP(res3, req3)
}

func Flashes(t *testing.T, newStore storeFactory) {
	r := fwncs.Default()
	store := newStore(t)
	r.Use(sessions.Sessions(sessionName, store))

	r.GET("/set", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.AddFlash(ok)
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/flash", func(c fwncs.Context) {
		session := sessions.Default(c)
		l := len(session.Flashes())
		if l != 1 {
			t.Error("Flashes count does not equal 1. Equals ", l)
		}
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/check", func(c fwncs.Context) {
		session := sessions.Default(c)
		l := len(session.Flashes())
		if l != 0 {
			t.Error("flashes count is not 0 after reading. Equals ", l)
		}
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/flash", nil)
	req2.Header.Set("Cookie", res1.Header().Get("Set-Cookie"))
	r.ServeHTTP(res2, req2)

	res3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/check", nil)
	req3.Header.Set("Cookie", res2.Header().Get("Set-Cookie"))
	r.ServeHTTP(res3, req3)
}

func Clear(t *testing.T, newStore storeFactory) {
	data := map[string]string{
		"key": "val",
		"foo": "bar",
	}
	r := fwncs.Default()
	store := newStore(t)
	r.Use(sessions.Sessions(sessionName, store))

	r.GET("/set", func(c fwncs.Context) {
		session := sessions.Default(c)
		for k, v := range data {
			session.Set(k, v)
		}
		session.Clear()
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/check", func(c fwncs.Context) {
		session := sessions.Default(c)
		for k, v := range data {
			if session.Get(k) == v {
				t.Fatal("Session clear failed")
			}
		}
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/check", nil)
	req2.Header.Set("Cookie", res1.Header().Get("Set-Cookie"))
	r.ServeHTTP(res2, req2)
}

func Options(t *testing.T, newStore storeFactory) {
	r := fwncs.Default()
	store := newStore(t)
	store.Options(sessions.Options{
		Domain: "localhost",
	})
	r.Use(sessions.Sessions(sessionName, store))

	r.GET("/domain", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Set("key", ok)
		session.Options(sessions.Options{
			Path: "/foo/bar/bat",
		})
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})
	r.GET("/path", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Set("key", ok)
		err := session.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	testOptionSameSitego(t, r)

	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/domain", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/path", nil)
	r.ServeHTTP(res2, req2)

	s := strings.Split(res1.Header().Get("Set-Cookie"), ";")
	assert.Equal(t, " Path=/foo/bar/bat", s[1], "Error writing path with options:", s[1])

	s = strings.Split(res2.Header().Get("Set-Cookie"), ";")
	assert.Equal(t, " Domain=localhost", s[1], "Error writing domain with options:", s[1])
}

func Many(t *testing.T, newStore storeFactory) {
	r := fwncs.Default()
	sessionNames := []string{"a", "b"}

	r.Use(sessions.SessionsMany(sessionNames, newStore(t)))

	r.GET("/set", func(c fwncs.Context) {
		sessionA := sessions.DefaultMany(c, "a")
		sessionA.Set("hello", "world")
		err := sessionA.Save()
		assert.NoError(t, err)
		sessionB := sessions.DefaultMany(c, "b")
		sessionB.Set("foo", "bar")
		err = sessionB.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	r.GET("/get", func(c fwncs.Context) {
		sessionA := sessions.DefaultMany(c, "a")
		assert.Equal(t, "world", sessionA.Get("hello"))
		err := sessionA.Save()
		assert.NoError(t, err)

		sessionB := sessions.DefaultMany(c, "b")
		assert.Equal(t, "bar", sessionB.Get("foo"))
		err = sessionB.Save()
		assert.NoError(t, err)
		c.String(http.StatusOK, ok)
	})

	res1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	r.ServeHTTP(res1, req1)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/get", nil)
	header := ""
	for _, x := range res1.Header()["Set-Cookie"] {
		header += strings.Split(x, ";")[0] + "; \n"
	}
	req2.Header.Set("Cookie", header)
	r.ServeHTTP(res2, req2)

}

func testOptionSameSitego(t *testing.T, r *fwncs.Router) {

	r.GET("/sameSite", func(c fwncs.Context) {
		session := sessions.Default(c)
		session.Set("key", ok)
		session.Options(sessions.Options{
			SameSite: http.SameSiteStrictMode,
		})
		err := session.Save()
		assert.NoError(t, err)
		c.String(200, ok)
	})

	res3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/sameSite", nil)
	r.ServeHTTP(res3, req3)

	s := strings.Split(res3.Header().Get("Set-Cookie"), ";")
	assert.Equal(t, " SameSite=Strict", s[1], "Error writing samesite with options:", s[1])
}
