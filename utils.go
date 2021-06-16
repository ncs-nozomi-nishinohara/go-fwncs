package fwncs

import (
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func NameOfFunction(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

type TransactionInfo struct {
	transactionName string
}

func (t *TransactionInfo) Name() string {
	return t.transactionName
}

func TransactionNameGenerator(router *Router) map[string]map[string]TransactionInfo {
	routes := router.routes
	rm := make(map[string]map[string]TransactionInfo)
	for method, value := range routes {
		mm := rm[method]
		if mm == nil {
			mm = make(map[string]TransactionInfo)
			rm[method] = mm
		}
		for _, v := range value {
			mm[v.handlerName] = TransactionInfo{
				transactionName: method + " " + v.path,
			}
		}
	}
	return rm
}

const PackageName = "github.com/n-creativesystem/fwncs"

func rewriteRulesRegex(rewrite map[string]string) map[*regexp.Regexp]string {
	rulesRegex := map[*regexp.Regexp]string{}
	for k, v := range rewrite {
		k = regexp.QuoteMeta(k)
		k = strings.Replace(k, `\*`, "(.*?)", -1)
		if strings.HasPrefix(k, `\^`) {
			k = strings.Replace(k, `\^`, "^", -1)
		}
		k = k + "$"
		rulesRegex[regexp.MustCompile(k)] = v
	}
	return rulesRegex
}

func rewriteURL(rewriteRegex map[*regexp.Regexp]string, req *http.Request) error {
	if len(rewriteRegex) == 0 {
		return nil
	}

	// Depending how HTTP request is sent RequestURI could contain Scheme://Host/path or be just /path.
	// We only want to use path part for rewriting and therefore trim prefix if it exists
	rawURI := req.RequestURI
	if rawURI != "" && rawURI[0] != '/' {
		prefix := ""
		if req.URL.Scheme != "" {
			prefix = req.URL.Scheme + "://"
		}
		if req.URL.Host != "" {
			prefix += req.URL.Host // host or host:port
		}
		if prefix != "" {
			rawURI = strings.TrimPrefix(rawURI, prefix)
		}
	}

	for k, v := range rewriteRegex {
		if replacer := captureTokens(k, rawURI); replacer != nil {
			url, err := req.URL.Parse(replacer.Replace(v))
			if err != nil {
				return err
			}
			req.URL = url

			return nil // rewrite only once
		}
	}
	return nil
}

func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil
	}
	values := groups[0][1:]
	replace := make([]string, 2*len(values))
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	return strings.NewReplacer(replace...)
}

func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}
