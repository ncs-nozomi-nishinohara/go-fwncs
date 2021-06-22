// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// at https://github.com/julienschmidt/httprouter/blob/master/LICENSE

package fwncs

import (
	"net/http"
	"regexp"
	"strings"
)

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is a Param-slice, as returned by the router.
// The slice is ordered, the first URL parameter is also the first slice value.
// It is therefore safe to read values by the index.
type Params []Param

// Get returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) Get(name string) (string, bool) {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value, true
		}
	}
	return "", false
}

// ByName returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) ByName(name string) (va string) {
	va, _ = ps.Get(name)
	return
}

func (ps Params) Values() []string {
	values := make([]string, len(ps))
	for i := range ps {
		values[i] = ps[i].Value
	}
	return values
}

type locationNode struct {
	reg          *regexp.Regexp
	originalPath string
	replacePath  string
	index        int
}

type locationNodes []locationNode

type paramNode struct {
	index     int
	params    *Params
	matchPath string
}

// Match
func (nodes locationNodes) Match(rawURI string) (param *paramNode) {
	var index = -1
	var max int = 0
	var groups [][]string
	for idx := range nodes {
		node := nodes[idx]
		v := node.reg.FindAllStringSubmatch(rawURI, -1)
		if v != nil {
			// 始めは無条件で設定
			if index == -1 {
				index = idx
				groups = v
			} else {
				// マッチした数が多い = パスの数が多いのでこちらを優先
				value := v[0]
				if len(value[1:]) > max && value[len(value)-1] != "" {
					max = len(v[0][1:])
					index = idx
					groups = v
				} else {
					if value[len(value)-1] != "" {
						oldPath := nodes[index].replacePath
						nowPath := node.replacePath
						if len(nowPath) > len(oldPath) {
							index = idx
							groups = v
						}
					}
				}
			}
		}
	}
	if index > -1 {
		node := nodes[index]
		originalUrl := node.replacePath
		matchValue := groups[0][1:]
		idxs := namedParam.FindAllStringIndex(originalUrl, -1)
		params := make(Params, len(idxs))
		for i, idx := range idxs {
			start := idx[0]
			end := idx[1]
			value := matchValue[i]
			if lastChar(value) == '/' {
				value = value[0 : len(value)-1]
			}
			params[i] = Param{
				Key:   originalUrl[start+1 : end],
				Value: value,
			}
		}
		param = &paramNode{
			index:     index,
			params:    &params,
			matchPath: node.originalPath,
		}
		return
	}
	return
}

type nodelocation struct {
	nodes  locationNodes
	full   locationNodes
	prefix locationNodes
}

var namedParam, _ = regexp.Compile(":[a-zA-Z0-9]+")

func locationRegex(paths []string) nodelocation {
	const (
		full   = "= "
		prefix = "~ "
	)
	var locations locationNodes
	var fullLocations locationNodes
	var prefixLocations locationNodes
	for idx, path := range paths {
		location := locationNode{
			index: idx,
		}
		if strings.HasPrefix(path, "= ") {
			path = strings.Replace(path, full, "", 1)
			location.originalPath = path
			fullLocations = append(fullLocations, location)
			continue
		}
		prefixFlg := strings.HasPrefix(path, "~ ")
		if prefixFlg {
			path = strings.Replace(path, prefix, "", 1)
		}
		location.originalPath = path
		path = regexp.QuoteMeta(path)
		path = strings.Replace(path, `\*`, ":", -1)
		path = strings.Replace(path, `\.`, ".", -1)
		if strings.HasPrefix(path, `\^`) {
			path = strings.Replace(path, `\^`, "^", -1)
		}
		location.replacePath = path
		path = namedParam.ReplaceAllString(path, "(.*?)")
		path = path + "$"
		location.reg = regexp.MustCompile(path)
		if prefixFlg {
			prefixLocations = append(prefixLocations, location)
			continue
		}
		locations = append(locations, location)
	}

	return nodelocation{
		nodes:  locations,
		full:   fullLocations,
		prefix: prefixLocations,
	}
}

/*
1.完全一致のURLを選択し、終了
2.優先したいlocationから正規表現で最も長い文字列でマッチしたもの
3.それ以外のlocationから正規表現で最も長い文字列でマッチしたもの
*/
func matchRequestURL(patterns nodelocation, req *http.Request) (node *paramNode) {
	rawURI := req.URL.Path
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
	return matchURL(patterns, rawURI)
}

func matchURL(patterns nodelocation, rawURI string) (node *paramNode) {
	for _, location := range patterns.full {
		if strings.EqualFold(rawURI, location.originalPath) {
			node = &paramNode{
				index:     location.index,
				params:    &Params{},
				matchPath: location.originalPath,
			}
			return
		}
	}
	node = patterns.prefix.Match(rawURI)
	if node == nil {
		node = patterns.nodes.Match(rawURI)
	}
	return
}
