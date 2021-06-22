# N-CreativeSystem Framework

Golang original framework

Inspired by [Gin](https://github.com/gin-gonic/gin) + [Echo](https://github.com/labstack/echo).

## Content

- [N-CreativeSystem Framework](#n-creativesystem-framework)
  - [Content](#content)
  - [Example](#example)

## Example

```go
package main

import (
    "github.com/n-creativesystem/go-fwncs"
)

func main() {
    router := fwncs.Default()
    router.GET("/", func(c fwncs.Context) {})
    api := router.Group("/api/v1")
    {
        api.GET("/resource/:name", func(c fwncs.Context) {})
        api.GET("= /resource/full-match", func(c fwncs.Context) {})
    }
    router.Run(8080) // or router.RunTLS(8443, "server.crt", "server.key")
}
```
