package etcdng

import (
	"encoding/json"
	"fmt"
	//"regexp"
	"strings"
	"time"

	"github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/log"
	"go.xpandmmi.com/xcms/core/engine"
)

type ng struct {
	nodes    []string
	etcdKey  string
	client   *etcd.Client
	cancelC  chan bool
	stopC    chan bool
	logsev   log.Severity

	options Options
}

type Options struct {
	EtcdConsistency string
	EtcdCaFile      string
	EtcdCertFile    string
	EtcdKeyFile     string
}

func New(nodes []string, etcdKey string, options Options) (engine.Engine, error) {
	n := &ng{
		nodes:    nodes,
		etcdKey:  etcdKey,
		cancelC:  make(chan bool, 1),
		stopC:    make(chan bool, 1),
		options:  setDefaults(options),
	}
	if err := n.reconnect(); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *ng) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

func (n *ng) GetLogSeverity() log.Severity {
	return n.logsev
}

func (n *ng) SetLogSeverity(sev log.Severity) {
	n.logsev = sev
	log.SetSeverity(n.logsev)
}

func (n *ng) reconnect() error {
	n.Close()
	var client *etcd.Client
	if n.options.EtcdCertFile == "" && n.options.EtcdKeyFile == "" {
		client = etcd.NewClient(n.nodes)
	} else {
		var err error
		if client, err = etcd.NewTLSClient(n.nodes, n.options.EtcdCertFile, n.options.EtcdKeyFile, n.options.EtcdCaFile); err != nil {
			return err
		}
	}
	if err := client.SetConsistency(n.options.EtcdConsistency); err != nil {
		return err
	}
	n.client = client
	return nil
}

// func (n *ng) sealJSONVal(val interface{}) ([]byte, error) {
// 	if n.options.Box == nil {
// 		return nil, fmt.Errorf("this backend does not support encryption")
// 	}
// 	bytes, err := json.Marshal(val)
// 	if err != nil {
// 		return nil, err
// 	}
// 	v, err := n.options.Box.Seal(bytes)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return secret.SealedValueToJSON(v)
// }


// Subscribe watches etcd changes and generates structured events telling vulcand to add or delete frontends, hosts etc.
// It is a blocking function.
func (n *ng) Subscribe(changes chan interface{}, cancelC chan bool) error {
	// This index helps us to get changes in sequence, as they were performed by clients.
	waitIndex := uint64(0)
	for {
		response, err := n.client.Watch(n.etcdKey, waitIndex, true, nil, cancelC)
		if err != nil {
			switch err {
			case etcd.ErrWatchStoppedByUser:
				log.Infof("Stop watching: graceful shutdown")
				return nil
			default:
				log.Errorf("unexpected error: %s, stop watching", err)
				return err
			}
		}
		waitIndex = response.Node.ModifiedIndex + 1
		log.Infof("%s", responseToString(response))
		change, err := n.parseChange(response)
		if err != nil {
			log.Warningf("Ignore '%s', error: %s", responseToString(response), err)
			continue
		}
		if change != nil {
			log.Infof("%v", change)
			select {
			case changes <- change:
			case <-cancelC:
				return nil
			}
		}
	}
}

type MatcherFn func(*etcd.Response) (interface{}, error)

// Dispatches etcd key changes changes to the etcd to the matching functions
func (s *ng) parseChange(response *etcd.Response) (interface{}, error) {
	// matchers := []MatcherFn{
	// 	// Host updates
	// 	s.parseHostChange,
  //
	// 	// Listener updates
	// 	s.parseListenerChange,
  //
	// 	// Frontend updates
	// 	s.parseFrontendChange,
	// 	s.parseFrontendMiddlewareChange,
  //
	// 	// Backend updates
	// 	s.parseBackendChange,
	// 	s.parseBackendServerChange,
	// }
	// for _, matcher := range matchers {
	// 	a, err := matcher(response)
	// 	if a != nil || err != nil {
	// 		return a, err
	// 	}
	// }
	return nil, nil
}
//
// func (n *ng) parseHostChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/hosts/([^/]+)(?:/host)?$").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 2 {
// 		return nil, nil
// 	}
//
// 	hostname := out[1]
//
// 	switch r.Action {
// 	case createA, setA:
// 		host, err := n.GetHost(engine.HostKey{hostname})
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.HostUpserted{
// 			Host: *host,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.HostDeleted{
// 			HostKey: engine.HostKey{hostname},
// 		}, nil
// 	}
// 	return nil, fmt.Errorf("unsupported action for host: %s", r.Action)
// }
//
// func (n *ng) parseListenerChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/listeners/([^/]+)").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 2 {
// 		return nil, nil
// 	}
//
// 	key := engine.ListenerKey{Id: out[1]}
//
// 	switch r.Action {
// 	case createA, setA:
// 		l, err := n.GetListener(key)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.ListenerUpserted{
// 			Listener: *l,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.ListenerDeleted{
// 			ListenerKey: key,
// 		}, nil
// 	}
// 	return nil, fmt.Errorf("unsupported action on the listener: %s", r.Action)
// }
//
// func (n *ng) parseFrontendChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/frontends/([^/]+)(?:/frontend)?$").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 2 {
// 		return nil, nil
// 	}
// 	key := engine.FrontendKey{Id: out[1]}
// 	switch r.Action {
// 	case createA, setA:
// 		f, err := n.GetFrontend(key)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.FrontendUpserted{
// 			Frontend: *f,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.FrontendDeleted{
// 			FrontendKey: key,
// 		}, nil
// 	case updateA: // this happens when we set TTL on a dir, ignore as there's no action needed from us
// 		return nil, nil
// 	}
// 	return nil, fmt.Errorf("unsupported action on the frontend: %v %v", r.Node.Key, r.Action)
// }
//
// func (s *ng) parseFrontendMiddlewareChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/frontends/([^/]+)/middlewares/([^/]+)$").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 3 {
// 		return nil, nil
// 	}
//
// 	fk := engine.FrontendKey{Id: out[1]}
// 	mk := engine.MiddlewareKey{FrontendKey: fk, Id: out[2]}
//
// 	switch r.Action {
// 	case createA, setA:
// 		m, err := s.GetMiddleware(mk)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.MiddlewareUpserted{
// 			FrontendKey: fk,
// 			Middleware:  *m,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.MiddlewareDeleted{
// 			MiddlewareKey: mk,
// 		}, nil
// 	}
// 	return nil, fmt.Errorf("unsupported action on the rate: %s", r.Action)
// }
//
// func (n *ng) parseBackendChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/backends/([^/]+)(?:/backend)?$").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 2 {
// 		return nil, nil
// 	}
// 	bk := engine.BackendKey{Id: out[1]}
// 	switch r.Action {
// 	case createA, setA:
// 		b, err := n.GetBackend(bk)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.BackendUpserted{
// 			Backend: *b,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.BackendDeleted{
// 			BackendKey: bk,
// 		}, nil
// 	}
// 	return nil, fmt.Errorf("unsupported node action: %s", r.Action)
// }
//
// func (n *ng) parseBackendServerChange(r *etcd.Response) (interface{}, error) {
// 	out := regexp.MustCompile("/backends/([^/]+)/servers/([^/]+)$").FindStringSubmatch(r.Node.Key)
// 	if len(out) != 3 {
// 		return nil, nil
// 	}
//
// 	sk := engine.ServerKey{BackendKey: engine.BackendKey{Id: out[1]}, Id: out[2]}
//
// 	switch r.Action {
// 	case setA, createA:
// 		srv, err := n.GetServer(sk)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &engine.ServerUpserted{
// 			BackendKey: sk.BackendKey,
// 			Server:     *srv,
// 		}, nil
// 	case deleteA, expireA:
// 		return &engine.ServerDeleted{
// 			ServerKey: sk,
// 		}, nil
// 	case cswapA: // ignore compare and swap attempts
// 		return nil, nil
// 	}
// 	return nil, fmt.Errorf("unsupported action on the server: %s", r.Action)
// }

func (n ng) path(keys ...string) string {
	return strings.Join(append([]string{n.etcdKey}, keys...), "/")
}

func (n *ng) setJSONVal(key string, v interface{}, ttl time.Duration) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return n.setVal(key, bytes, ttl)
}

func (n *ng) setVal(key string, val []byte, ttl time.Duration) error {
	_, err := n.client.Set(key, string(val), uint64(ttl/time.Second))
	return convertErr(err)
}

func (n *ng) getJSONVal(key string, in interface{}) error {
	val, err := n.getVal(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), in)
}

func (n *ng) getVal(key string) (string, error) {
	response, err := n.client.Get(key, false, false)
	if err != nil {
		return "", convertErr(err)
	}

	if isDir(response.Node) {
		return "", &engine.NotFoundError{Message: fmt.Sprintf("missing key: %s", key)}
	}
	return response.Node.Value, nil
}

func (n *ng) getDirs(keys ...string) ([]string, error) {
	var out []string
	response, err := n.client.Get(strings.Join(keys, "/"), true, true)
	if err != nil {
		if notFound(err) {
			return out, nil
		}
		return nil, err
	}

	if response == nil || !isDir(response.Node) {
		return out, nil
	}

	for _, srvNode := range response.Node.Nodes {
		if isDir(srvNode) {
			out = append(out, srvNode.Key)
		}
	}
	return out, nil
}

func (n *ng) getVals(keys ...string) ([]Pair, error) {
	var out []Pair
	response, err := n.client.Get(strings.Join(keys, "/"), true, true)
	if err != nil {
		if notFound(err) {
			return out, nil
		}
		return nil, err
	}

	if !isDir(response.Node) {
		return out, nil
	}

	for _, srvNode := range response.Node.Nodes {
		if !isDir(srvNode) {
			out = append(out, Pair{srvNode.Key, srvNode.Value})
		}
	}
	return out, nil
}

func (n *ng) checkKeyExists(key string) error {
	_, err := n.client.Get(key, false, false)
	return convertErr(err)
}

func (n *ng) deleteKey(key string) error {
	_, err := n.client.Delete(key, true)
	return convertErr(err)
}

type Pair struct {
	Key string
	Val string
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

func join(keys ...string) string {
	return strings.Join(keys, "/")
}

func notFound(e error) bool {
	err, ok := e.(*etcd.EtcdError)
	return ok && err.ErrorCode == 100
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case *etcd.EtcdError:
		if err.ErrorCode == 100 {
			return &engine.NotFoundError{Message: err.Error()}
		}
		if err.ErrorCode == 105 {
			return &engine.AlreadyExistsError{Message: err.Error()}
		}
	}
	return e
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}

func isNotFoundError(err error) bool {
	_, ok := err.(*engine.NotFoundError)
	return ok
}

const encryptionSecretBox = "secretbox.v1"

func responseToString(r *etcd.Response) string {
	return fmt.Sprintf("%s %s %d", r.Action, r.Node.Key, r.EtcdIndex)
}

const (
	createA = "create"
	setA    = "set"
	deleteA = "delete"
	expireA = "expire"
	updateA = "update"
	cswapA  = "compareAndSwap"
	noTTL   = 0
)

func setDefaults(o Options) Options {
	if o.EtcdConsistency == "" {
		o.EtcdConsistency = etcd.STRONG_CONSISTENCY
	}
	return o
}

type host struct {
	Name     string
	Settings hostSettings
}

type hostSettings struct {
	Default bool
	KeyPair []byte
	//OCSP    engine.OCSPSettings
}
