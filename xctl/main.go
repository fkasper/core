package main

import (
	"fmt"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/gocql/gocql"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/log"
	elastigo "github.com/fkasper/core/Godeps/_workspace/src/github.com/mattbaird/elastigo/lib"
	"os"
	"time"
  "strings"
	//"encoding/json"
)

type IntellipediaPage struct {
  Url               string             `json:"url"`
  Content           string             `json:"content"`
  Image             string             `json:"image"`
  PreviewContent    string             `json:"previewcontent"`
  Title             string             `json:"title"`
}
type SocialSite struct {
  Name            string            `json:"name"`
  Link            string            `json:"link"`
  Tile            string            `json:"tile"`
}
type NewsRecord struct {
	Url            string             `json:"url"`
	NewsId         gocql.UUID         `json:"news_id"`
	Content        string             `json:"content"`
	Media          map[string]*string `json:"media"`
	PostedAt       time.Time          `json:"posted_at"`
	PreviewContent string             `json:"previewcontent"`
	Title          string             `json:"title"`
	Link           string             `json:"link"`
}
type Record struct {
  Title string                      `json:"title"`
  Preview string                    `json:"preview"`
  Image string                      `json:"image"`
  Link string                       `json:"link"`
  Content        string             `json:"content"`
	Media          map[string]*string `json:"media"`
	PostedAt       time.Time          `json:"posted_at"`
}

func main() {
	app := cli.NewApp()
	app.Name = "XTOOLS for SITREP"
	app.Usage = "Tools for the SITREP Server Environment"
	app.Commands = []cli.Command{
		{
			Name:    "index",
			Aliases: []string{"i"},
			Usage:   "Re-Index everything into Elasticsearch",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "cassandra, c",
					Value: "127.0.0.1",
					Usage: "Cassandra Endpoint",
				},
				cli.StringFlag{
					Name:  "keyspace, k",
					Value: "sitrep_dev",
					Usage: "SITREP ENDPOINT",
				},
				cli.StringFlag{
					Name:  "elastichost, e",
					Value: "elasticsearch.dev.docker",
					Usage: "Elasticsearch Host",
				},
			},
			Action: func(c *cli.Context) {
				db := gocql.NewCluster(c.String("cassandra"))
				db.Keyspace = "sitrep_dev"
				db.DiscoverHosts = true
				db.Discovery = gocql.DiscoveryConfig{
					DcFilter:   "",
					RackFilter: "",
					Sleep:      30 * time.Second,
				}
				session, _ := db.CreateSession()
				defer session.Close()

				news := NewsRecord{}
				cl := elastigo.NewConn()
				cl.Domain = c.String("elastichost")

				iter := session.Query(`SELECT url, newsid, content, media, postedat, previewcontent, title FROM newsstoriestable`).Iter()
				for iter.Scan(&news.Url,
					&news.NewsId,
					&news.Content,
					&news.Media,
					&news.PostedAt,
					&news.PreviewContent,
					&news.Title) {
          item := &Record{
            Title: news.Title,
            Preview: news.PreviewContent,
            Image: *news.Media["preview"],
            Link: "/news/flobama/"+news.Url,
            Content: news.Content,
            PostedAt: news.PostedAt,
            Media: news.Media,
          }
					cl.Index("sites", "news", news.Url, nil, item)
					fmt.Println("Indexed:", news.Url)
				}
				if err := iter.Close(); err != nil {
					log.Errorf(err.Error())
				}
        page := IntellipediaPage{}
        iter2 := session.Query(`SELECT url, content, image, mapimg, name FROM intellipediapagestable`).Iter()
				for iter2.Scan(&page.Url,
					&page.Content,
					&page.Image,
					&page.PreviewContent,
					&page.Title) {
          item := &Record{
            Title: page.Title,
            Preview: page.PreviewContent,
            Image: page.Image,
            Link: "/intellipedia/"+page.Url,
            Content: page.Content,
          }
					cl.Index("sites", "intellipedia", page.Url, nil, item)
					fmt.Println("Indexed:", page.Url)
				}
				if err := iter2.Close(); err != nil {
					log.Errorf(err.Error())
				}
        site := SocialSite{}
        iter3 := session.Query(`SELECT name, link, tile FROM categoriestable`).Iter()
				for iter3.Scan(&site.Name,
          &site.Link,
          &site.Tile) {
          item := &Record{
            Title: site.Name,
            Preview: "",
            Image: site.Tile,
            Link: site.Link,
          }
					cl.Index("sites", "sites", strings.Replace(site.Name, " ", "-", -1), nil, item)
					fmt.Println("Indexed:", site.Name)
				}
				if err := iter3.Close(); err != nil {
					log.Errorf(err.Error())
				}
			},
		},
		{
			Name:    "complete",
			Aliases: []string{"c"},
			Usage:   "complete a task on the list",
			Action: func(c *cli.Context) {
				println("completed task: ", c.Args().First())
			},
		},
		{
			Name:    "template",
			Aliases: []string{"r"},
			Usage:   "options for task templates",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new template",
					Action: func(c *cli.Context) {
						println("new task template: ", c.Args().First())
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing template",
					Action: func(c *cli.Context) {
						println("removed task template: ", c.Args().First())
					},
				},
			},
		},
	}

	app.Run(os.Args)
}
