package engine

import (
	//"time"

	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/log"
//	elastigo "github.com/fkasper/core/Godeps/_workspace/src/github.com/mattbaird/elastigo/lib"
)

type NewEngineFn func() (Engine, error)

// Engine is an interface for storage and configuration engine, e.g. Etcd.
// Simple in memory implementation is available at engine/memng package
// Engines should pass the following acceptance suite to be compatible:
// engine/test/suite.go, see engine/etcdng/etcd_test.go and engine/memng/mem_test.go for details
type Engine interface {

	// Subscribe is an entry point for getting the configuration changes as well as the initial configuration.
	// It should be a blocking function generating events from change.go to the changes channel.
	// Each change should be an instance of the struct provided in events.go
	// In  case if cancel channel is closed, the subscribe events should no longer be generated.
	Subscribe(events chan interface{}, cancel chan bool) error

	IssueAuthenticationToken(hostname string, email string, password string) (string, error)
  SearchAll(limit string, query string) (*SearchResult, error)
  ValidateTokenRequest(token string) error

	// GetLogSeverity returns the current logging severity level
	GetLogSeverity() log.Severity
	// SetLogSeverity updates the logging severity level
	SetLogSeverity(log.Severity)

	// Close should close all underlying resources such as connections, files, etc.
	Close()
}

type Result struct {
  Title interface{}
  Preview interface{}
  Image interface{}
  Link interface{}
}

type SearchResult struct {
  Intellipedia  []*Result
  News          []*Result
  Sites         []*Result
}

type SiteSetting struct {
	Site          string
	MetaStorage   map[string]*string
	ClientStorage map[string]*string
	PluginStorage map[string]map[string]*string
}
