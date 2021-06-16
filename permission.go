package fwncs

import (
	"net/http"

	"github.com/form3tech-oss/jwt-go"
)

type permissionMap map[string]bool

func (p *permissionMap) IsDenied() bool {
	for _, val := range *p {
		if !val {
			return true
		}
	}
	return false
}

func (p *permissionMap) copy() permissionMap {
	mp := permissionMap{}
	for key := range *p {
		mp[key] = false
	}
	return mp
}

func newPermissionMap(permissions ...string) permissionMap {
	mpPermission := permissionMap{}
	for _, permission := range permissions {
		mpPermission[permission] = false
	}
	return mpPermission
}

func Permission(permissionClaim string, permissions ...string) HandlerFunc {
	originalPermission := newPermissionMap(permissions...)
	return func(c Context) {
		mpPermission := originalPermission.copy()
		token, ok := c.Get(AuthKey).(*jwt.Token)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
		}
		mpClaims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
		}
		scopes := getScope(mpClaims[permissionClaim])
		for _, scope := range scopes {
			if _, ok := mpPermission[scope]; ok {
				mpPermission[scope] = true
			}
		}
		if mpPermission.IsDenied() {
			c.AbortWithStatus(http.StatusForbidden)
		}
		if !c.IsAbort() {
			c.Next()
		} else {
			c.SetHeader("WWW-Authenticate", "Bearer error=\"insufficient_scope\"")
		}
	}
}

func getScope(value interface{}) []string {
	scopes := make([]string, 0)
	switch v := value.(type) {
	case []interface{}:
		for _, vv := range v {
			scopes = append(scopes, getScope(vv)...)
		}
	case []string:
		scopes = append(scopes, v...)
	case string:
		scopes = append(scopes, v)
	}
	return scopes
}
