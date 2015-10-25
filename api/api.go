package api

import (
	//"encoding/json"
	//"fmt"
	"net/http"
	// "strconv"
	// "time"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/scroll"
	"github.com/fkasper/core/engine"
	"html/template"
)

type ProxyController struct {
	ng  engine.Engine
	app *scroll.App
}

var templates = template.Must(template.ParseGlob("templates/**"))


func InitProxyController(ng engine.Engine, app *scroll.App) {
	c := &ProxyController{ng: ng, app: app}

	app.SetNotFoundHandler(c.handleError)

	app.AddHandler(scroll.Spec{Paths: []string{"/services/status"}, Methods: []string{"GET"}, HandlerWithBody: c.getStatus})
	app.AddHandler(scroll.Spec{Paths: []string{"/services/oauth2/token"}, Methods: []string{"POST"}, HandlerWithBody: c.signInUser})
	app.AddHandler(scroll.Spec{Paths: []string{"/services/search"}, Methods: []string{"GET"}, RawHandler: c.search})
  app.AddHandler(scroll.Spec{Paths: []string{"/services/twitter/form"}, Methods: []string{"GET"}, RawHandler: c.tweetForm})
}

func (c *ProxyController) getStatus(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return scroll.Response{
		"Status": "ok",
	}, nil
}

func renderForbidden(w http.ResponseWriter, r *http.Request, reason interface{}) {
  if err := templates.ExecuteTemplate(w, "forbidden", reason); err != nil {
    w.Write([]byte("An error occured"))
    return
  }
}

func renderTweetForm(w http.ResponseWriter, r *http.Request, reason interface{}) {
  if err := templates.ExecuteTemplate(w, "tweetForm", reason); err != nil {
    w.Write([]byte("An error occured"))
    return
  }
}

func (c *ProxyController) tweetForm(w http.ResponseWriter, r *http.Request) {
  w.Header().Add("Content-Type", "text/html; charset=utf-8")
  queryString := r.URL.Query().Get("token")
  if queryString == "" {
    renderForbidden(w, r, "This token is invalid. Please request a new one")
    return
  }
  if err := c.ng.ValidateTokenRequest(queryString); err != nil {
    renderForbidden(w, r, "This token is invalid. Please request a new one" + err.Error())
    return
  }

  renderTweetForm(w,r,"OK")
}

func (c *ProxyController) search(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	queryString := r.URL.Query().Get("q")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "30"
	}
	searchResult, err := c.ng.SearchAll(limit, queryString)
	if err != nil {
		w.Write([]byte(err.Error()))
    return
	}
	if err := templates.ExecuteTemplate(w, "searchResult", searchResult); err != nil {
		w.Write([]byte("An error occured"))
    return
	}

	// if err != nil {
	// 	return nil, formatError(err)
	// }
	// return scroll.Response{
	// 	"access_token": accessToken,
	// 	"token_type": "bearer",
	// }, nil
}

func (c *ProxyController) signInUser(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	grantType := r.PostFormValue("grant_type")
	if grantType != "password" {
		return nil, formatError(scroll.InvalidFormatError{
			Field: "grant_type",
			Value: "must be type password on this endpoint",
		})
	}
	email := r.PostFormValue("email")
	password := r.PostFormValue("password")
	if email == "" || password == "" {
		return nil, formatError(scroll.InvalidFormatError{
			Field: "email,password",
			Value: "you must supply username and password",
		})
	}
	accessToken, err := c.ng.IssueAuthenticationToken(r.Host, email, password)
	if err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{
		"access_token": accessToken,
		"token_type":   "bearer",
	}, nil
}

func (c *ProxyController) handleError(w http.ResponseWriter, r *http.Request) {
	log.Infof("not found")
	scroll.ReplyError(w, scroll.NotFoundError{Description: "Object not found"})
}

func formatError(e error) error {
	switch err := e.(type) {
	case *engine.AlreadyExistsError:
		return scroll.ConflictError{Description: err.Error()}
	case *engine.NotFoundError:
		return scroll.NotFoundError{Description: err.Error()}
	case *engine.InvalidFormatError:
		return scroll.InvalidParameterError{Value: err.Error()}
	case scroll.GenericAPIError, scroll.MissingFieldError,
		scroll.InvalidFormatError, scroll.InvalidParameterError,
		scroll.NotFoundError, scroll.ConflictError:
		return e
	}
	return scroll.GenericAPIError{Reason: e.Error()}
}

func formatResult(in interface{}, err error) (interface{}, error) {
	if err != nil {
		return nil, formatError(err)
	}
	return in, nil
}
