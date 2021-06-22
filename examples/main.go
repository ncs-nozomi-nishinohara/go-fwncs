package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

func init() {
	viper.SetConfigFile(`config.json`)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if viper.GetBool(`debug`) {
		fmt.Println("Service RUN on DEBUG mode")
	}
}

type GitHubLang struct {
	Name  string `json:"name" validate:"required"`
	Color string `json:"color" validate:"required"`
	Size  int    `json:"size"`
}

type Language struct {
	Name  string
	Color string
}

type Repository struct {
	Name      string
	Languages struct {
		TotalSize int
		Edges     []struct {
			Size int
			Node struct {
				Language `graphql:"... on Language"`
			}
		}
	} `graphql:"languages(first: 100)"`
}

type Query struct {
	Search struct {
		Nodes []struct {
			Repository `graphql:"... on Repository"`
		}
	} `graphql:"search(first: 100, query: $q, type: REPOSITORY)"`
}

type apiGitHub struct {
	Client Client
}

type Client interface {
	GetColor(username string) (error, []GitHubLang)
	CallApi(username string) (error, *Query)
}

func (github *apiGitHub) DoGetColor(username string) (error, []GitHubLang) {
	return github.Client.GetColor(username)
}

func (github *apiGitHub) DoCallApi(username string) (error, *Query) {
	return github.Client.CallApi(username)
}

var query = Query{}

type GithubClientImpl struct {
}

func (client *GithubClientImpl) GetColor(username string) (error, []GitHubLang) {
	err, tmpQuery := client.CallApi(username)
	if err != nil {
		return err, nil
	}
	var langs []GitHubLang
	for _, repo := range tmpQuery.Search.Nodes {
		for _, lang := range repo.Languages.Edges {
			isContain, i := langsContains(langs, lang.Node.Name)
			if isContain {
				langs[i].Size = lang.Size + langs[i].Size
			} else {
				langs = append(langs, GitHubLang{Name: lang.Node.Name, Size: lang.Size, Color: lang.Node.Color})
			}
		}
	}
	return nil, langs
}

func (client *GithubClientImpl) CallApi(username string) (error, *Query) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: viper.GetString("github.token")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	cc := githubv4.NewClient(httpClient)
	variables := map[string]interface{}{
		"q": githubv4.String("user:" + username),
	}
	err := cc.Query(context.Background(), &query, variables)
	if err != nil {
		return err, nil
	}
	return nil, &query
}

func langsContains(arr []GitHubLang, str string) (bool, int) {
	for i, v := range arr {
		if v.Name == str {
			return true, i
		}
	}
	return false, -1
}

func main() {
	router := fwncs.Default()
	router.GET("/get/:username", func(c fwncs.Context) {
		username := c.Param("username")
		github := &apiGitHub{Client: &GithubClientImpl{}}
		err, langs := github.DoGetColor(username)
		if err != nil {
			c.Error(err)
			c.AbortWithStatus(http.StatusInternalServerError)
		} else {
			c.JSON(http.StatusOK, langs)
		}
	})
	router.RunTLS(8080, "../tests/server.crt", "../tests/server.key")
}
