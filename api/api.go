package api

import (
	//"encoding/json"
	//"fmt"
	"net/http"
	// "strconv"
	// "time"

	"github.com/mailgun/log"
	"github.com/mailgun/scroll"
  "go.xpandmmi.com/xcms/core/engine"
)

type ProxyController struct {
	ng    engine.Engine
	app   *scroll.App
}

func InitProxyController(ng engine.Engine, app *scroll.App) {
	c := &ProxyController{ng: ng, app: app}

	app.SetNotFoundHandler(c.handleError)

  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/status"}, Methods: []string{"GET"}, HandlerWithBody: c.getStatus})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/sitename"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/settings/public"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/pages/{page}/meta"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/pages/{page}/comments"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/pages"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/api/v1/settings/public"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})
  app.AddHandler(scroll.Spec{Paths: []string{"/attachments/{date}/{attachment}"}, Methods: []string{"GET"}, HandlerWithBody: c.getSiteName})

}

func (c *ProxyController) getSiteName(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return scroll.Response{
		"Hostname": r.Host,
	}, nil
}

func (c *ProxyController) getStatus(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return scroll.Response{
		"Status": "ok",
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
