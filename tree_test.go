package fwncs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	url    string
	match  bool
	path   string
	params Params
}

func checkRequests(t *testing.T, location nodelocation, testCases []testCase) {
	for idx, testCase := range testCases {
		req := httptest.NewRequest(http.MethodGet, testCase.url, nil)
		node := matchRequestURL(location, req)
		if testCase.match {
			if assert.NotNil(t, node, idx) {
				assert.Equal(t, testCase.params, *node.params, idx)
				assert.Equal(t, testCase.path, node.matchPath, idx)
			}
		}
	}
}

func TestRewrite(t *testing.T) {
	paths := []string{
		"/abc/*path",
		"/abc/*name/abcd/*param",
		"/api/*name",
		"= /api/v1/test",
	}
	locations := locationRegex(paths)
	testCases := []testCase{
		{
			url:    "/abc/v1?aaa=bbb",
			match:  true,
			params: Params{Param{"path", "v1"}},
			path:   "/abc/*path",
		},
		{
			url:    "/abc/v1/abcd/aaaa",
			match:  true,
			params: Params{Param{"name", "v1"}, Param{"param", "aaaa"}},
			path:   "/abc/*name/abcd/*param",
		},
		{
			url:   "/cccc",
			match: false,
			path:  "",
		},
		{
			url:   "/ann/v1",
			match: false,
			path:  "",
		},
		{
			url:    "/api/v1",
			match:  true,
			params: Params{Param{"name", "v1"}},
			path:   "/api/*name",
		},
		{
			url:    "/api/v1/test",
			match:  true,
			params: Params{},
			path:   "/api/v1/test",
		},
	}
	checkRequests(t, locations, testCases)
}

func TestTreeAddAndGet(t *testing.T) {
	paths := [...]string{
		"/",
		"/cmd/:tool",
		"/cmd/:tool/",
		"/cmd/:tool/:sub",
		"/cmd/whoami",
		"/cmd/whoami/root",
		"/cmd/whoami/root/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/search/go-fwncs",
		"/search/google",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
		"/info/:user/project/golang",
	}
	locations := locationRegex(paths[:])
	case_ := []testCase{
		{"/", false, "/", Params{}},
		{"/cmd/test", true, "/cmd/:tool", Params{Param{"tool", "test"}}},
		{"/cmd/test/", true, "/cmd/:tool/", Params{Param{"tool", "test"}}},
		{"/cmd/test/3", true, "/cmd/:tool/:sub", Params{Param{Key: "tool", Value: "test"}, Param{Key: "sub", Value: "3"}}},
		{"/cmd/who", true, "/cmd/:tool", Params{Param{"tool", "who"}}},
		{"/cmd/who/", true, "/cmd/:tool/", Params{Param{"tool", "who"}}},
		{"/cmd/whoami", true, "/cmd/whoami", Params{}},
		{"/cmd/whoami/", false, "/cmd/whoami", Params{}},
		{"/cmd/whoami/r", true, "/cmd/:tool/:sub", Params{Param{Key: "tool", Value: "whoami"}, Param{Key: "sub", Value: "r"}}},
		{"/cmd/whoami/r/", true, "/cmd/:tool/:sub", Params{Param{Key: "tool", Value: "whoami"}, Param{Key: "sub", Value: "r"}}},
		{"/cmd/whoami/root", false, "/cmd/whoami/root", Params{}},
		{"/cmd/whoami/root/", false, "/cmd/whoami/root/", Params{}},
		{"/src/", false, "/src/*filepath", Params{}},
		{"/src/some/file.png", true, "/src/*filepath", Params{Param{Key: "filepath", Value: "some/file.png"}}},
		{"/search/", false, "/search/", Params{}},
		{"/search/someth!ng+in+ünìcodé", true, "/search/:query", Params{Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}},
		{"/search/someth!ng+in+ünìcodé/", true, "/search/:query", Params{Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}},
		{"/search/fwncs", true, "/search/:query", Params{Param{"query", "fwncs"}}},
		{"/search/go-fwncs", false, "/search/go-fwncs", Params{}},
		{"/search/google", false, "/search/google", Params{}},
		{"/user_gopher", true, "/user_:name", Params{Param{Key: "name", Value: "gopher"}}},
		{"/user_gopher/about", true, "/user_:name/about", Params{Param{Key: "name", Value: "gopher"}}},
		{"/files/js/inc/framework.js", true, "/files/:dir/*filepath", Params{Param{Key: "dir", Value: "js"}, Param{Key: "filepath", Value: "inc/framework.js"}}},
		{"/info/gordon/public", true, "/info/:user/public", Params{Param{Key: "user", Value: "gordon"}}},
		{"/info/gordon/project/go", true, "/info/:user/project/:project", Params{Param{Key: "user", Value: "gordon"}, Param{Key: "project", Value: "go"}}},
		{"/info/gordon/project/golang", true, "/info/:user/project/golang", Params{Param{Key: "user", Value: "gordon"}}},
	}
	checkRequests(t, locations, case_)
}
