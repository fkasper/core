package etcdng

import (
	"encoding/json"
	"fmt"
	//"regexp"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/dgrijalva/jwt-go"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/gocql/gocql"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/log"
	elastigo "github.com/fkasper/core/Godeps/_workspace/src/github.com/mattbaird/elastigo/lib"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mattes/migrate/migrate"
	"github.com/fkasper/core/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/fkasper/core/engine"
	"math/rand"
	"strings"
	"time"
)

type ng struct {
	nodes    []string
	etcdKey  string
	client   *etcd.Client
	cancelC  chan bool
	stopC    chan bool
	logsev   log.Severity
	Database *gocql.ClusterConfig
	options  Options
}

type Options struct {
	EtcdConsistency string
	EtcdCaFile      string
	EtcdCertFile    string
	EtcdKeyFile     string
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func New(nodes []string, etcdKey string, options Options) (engine.Engine, error) {
	n := &ng{
		nodes:   nodes,
		etcdKey: etcdKey,
		cancelC: make(chan bool, 1),
		stopC:   make(chan bool, 1),
		options: setDefaults(options),
	}
	if err := n.reconnect(); err != nil {
		return nil, err
	}
	if err := n.buildDbCluster(); err != nil {
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

func (n *ng) buildDbCluster() error {
	dbEndpoints := n.path("cassandra", "hosts")

	var cassandraPool *cassandraPool
	err := n.getJSONVal(dbEndpoints, &cassandraPool)
	if err != nil {
		return err
	}

	log.Infof("===> Syncing up Migrations with the cluster")
	allErr, ok := migrate.UpSync("cassandra://"+cassandraPool.Nodes[0]+"/"+cassandraPool.Keyspace, "./migrations")
	if !ok {
		return allErr[0]
	}
	log.Infof("===> Migrations are in sync")
	log.Infof("===> Connecting to the database cluster")

	db := gocql.NewCluster(cassandraPool.Nodes...)
	db.Keyspace = cassandraPool.Keyspace
	db.NumConns = cassandraPool.NumConns
	db.DiscoverHosts = true
	db.Discovery = gocql.DiscoveryConfig{
		DcFilter:   "",
		RackFilter: "",
		Sleep:      30 * time.Second,
	}
	n.Database = db
	log.Infof("===> Connected to the database cluster")

	return nil
}

func (n *ng) GetUserByEmail(email string) (MinimalUserRecord, error) {
	user := MinimalUserRecord{}
	session, _ := n.Database.CreateSession()
	defer session.Close()
	log.Debugf("===> Querying Database for email: " + email)
	if err := session.Query(`SELECT id,
    email,
    hashed_password,
    password_salt,
    encryption_keys,
    meta_storage,
    is_active
    FROM access_users
    WHERE email = ?
    LIMIT 1 ALLOW FILTERING`,
		email).Consistency(gocql.One).Scan(&user.Id,
		&user.Email,
		&user.HashedPassword,
		&user.PasswordSalt,
		&user.EncryptionKeys,
		&user.MetaStorage,
		&user.IsActive); err != nil {
		log.Errorf("===> Got a database error while querying for specific user")
		return MinimalUserRecord{}, err
	}
	return user, nil
}

var src = rand.NewSource(time.Now().UnixNano())

func RandStringBytesMaskImprSrc(n int) []byte {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return b
}

func (n *ng) IssueAuthenticationToken(hostname string, email string, password string) (string, error) {
	session, _ := n.Database.CreateSession()
	defer session.Close()
	user, err := n.GetUserByEmail(email)

	if err != nil {
		return "", err
	}

	if user.IsActive != true {
		return "", fmt.Errorf("User is not active")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)); err != nil {
		return "", fmt.Errorf("Invalid Password")
	}

	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims["sub"] = user.Id
	token.Claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	return token.SignedString(RandStringBytesMaskImprSrc(40))
	//1. verify domain (active)
	//2. verify customer (active)
	//3. verify user (active -> password)
	//4. issue token and log to database
	//n.GetFullUserForUsername(username)
	//n.GetCustomerForDomain(hostname)
	//if err := session.Query(`SELECT site,`); err != nil {
	//  log.Errorf("===> Got a database error while querying for site_settings")
	//  return "", err
	//}
}

func (n *ng) Search(token string, limit string, query string) (elastigo.Hits, error) {
	c := elastigo.NewConn()
	c.SetHosts([]string{"elasticsearch.dev.docker"})
	searchJson := `{
        "from" : 0, "size" : ` + limit + `,
  	    "query" : {
          "multi_match" : {
            "query":    "` + query + `",
            "fields": [ "title^3", "name^3", "url^2", "content", "preview" ]
          }
  	    }
  	}`
	out, err := c.Search("sites", token, nil, searchJson)
	if err != nil {
		return elastigo.Hits{}, err
	}
	return out.Hits, nil
}

func (n *ng) JSONVal(val interface{}) ([]byte, error) {
	bytes, err := json.Marshal(val)
	return bytes, err
}

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
	//   // Host updates
	//   s.parseHostChange,
	//
	//   // Listener updates
	//   s.parseListenerChange,
	//
	//   // Frontend updates
	//   s.parseFrontendChange,
	//   s.parseFrontendMiddlewareChange,
	//
	//   // Backend updates
	//   s.parseBackendChange,
	//   s.parseBackendServerChange,
	// }
	// for _, matcher := range matchers {
	//   a, err := matcher(response)
	//   if a != nil || err != nil {
	//     return a, err
	//   }
	// }
	return nil, nil
}

//
// func (n *ng) parseHostChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/hosts/([^/]+)(?:/host)?$").FindStringSubmatch(r.Node.Key)
//   if len(out) != 2 {
//     return nil, nil
//   }
//
//   hostname := out[1]
//
//   switch r.Action {
//   case createA, setA:
//     host, err := n.GetHost(engine.HostKey{hostname})
//     if err != nil {
//       return nil, err
//     }
//     return &engine.HostUpserted{
//       Host: *host,
//     }, nil
//   case deleteA, expireA:
//     return &engine.HostDeleted{
//       HostKey: engine.HostKey{hostname},
//     }, nil
//   }
//   return nil, fmt.Errorf("unsupported action for host: %s", r.Action)
// }
//
// func (n *ng) parseListenerChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/listeners/([^/]+)").FindStringSubmatch(r.Node.Key)
//   if len(out) != 2 {
//     return nil, nil
//   }
//
//   key := engine.ListenerKey{Id: out[1]}
//
//   switch r.Action {
//   case createA, setA:
//     l, err := n.GetListener(key)
//     if err != nil {
//       return nil, err
//     }
//     return &engine.ListenerUpserted{
//       Listener: *l,
//     }, nil
//   case deleteA, expireA:
//     return &engine.ListenerDeleted{
//       ListenerKey: key,
//     }, nil
//   }
//   return nil, fmt.Errorf("unsupported action on the listener: %s", r.Action)
// }
//
// func (n *ng) parseFrontendChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/frontends/([^/]+)(?:/frontend)?$").FindStringSubmatch(r.Node.Key)
//   if len(out) != 2 {
//     return nil, nil
//   }
//   key := engine.FrontendKey{Id: out[1]}
//   switch r.Action {
//   case createA, setA:
//     f, err := n.GetFrontend(key)
//     if err != nil {
//       return nil, err
//     }
//     return &engine.FrontendUpserted{
//       Frontend: *f,
//     }, nil
//   case deleteA, expireA:
//     return &engine.FrontendDeleted{
//       FrontendKey: key,
//     }, nil
//   case updateA: // this happens when we set TTL on a dir, ignore as there's no action needed from us
//     return nil, nil
//   }
//   return nil, fmt.Errorf("unsupported action on the frontend: %v %v", r.Node.Key, r.Action)
// }
//
// func (s *ng) parseFrontendMiddlewareChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/frontends/([^/]+)/middlewares/([^/]+)$").FindStringSubmatch(r.Node.Key)
//   if len(out) != 3 {
//     return nil, nil
//   }
//
//   fk := engine.FrontendKey{Id: out[1]}
//   mk := engine.MiddlewareKey{FrontendKey: fk, Id: out[2]}
//
//   switch r.Action {
//   case createA, setA:
//     m, err := s.GetMiddleware(mk)
//     if err != nil {
//       return nil, err
//     }
//     return &engine.MiddlewareUpserted{
//       FrontendKey: fk,
//       Middleware:  *m,
//     }, nil
//   case deleteA, expireA:
//     return &engine.MiddlewareDeleted{
//       MiddlewareKey: mk,
//     }, nil
//   }
//   return nil, fmt.Errorf("unsupported action on the rate: %s", r.Action)
// }
//
// func (n *ng) parseBackendChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/backends/([^/]+)(?:/backend)?$").FindStringSubmatch(r.Node.Key)
//   if len(out) != 2 {
//     return nil, nil
//   }
//   bk := engine.BackendKey{Id: out[1]}
//   switch r.Action {
//   case createA, setA:
//     b, err := n.GetBackend(bk)
//     if err != nil {
//       return nil, err
//     }
//     return &engine.BackendUpserted{
//       Backend: *b,
//     }, nil
//   case deleteA, expireA:
//     return &engine.BackendDeleted{
//       BackendKey: bk,
//     }, nil
//   }
//   return nil, fmt.Errorf("unsupported node action: %s", r.Action)
// }
//
// func (n *ng) parseBackendServerChange(r *etcd.Response) (interface{}, error) {
//   out := regexp.MustCompile("/backends/([^/]+)/servers/([^/]+)$").FindStringSubmatch(r.Node.Key)
//   if len(out) != 3 {
//     return nil, nil
//   }
//
//   sk := engine.ServerKey{BackendKey: engine.BackendKey{Id: out[1]}, Id: out[2]}
//
//   switch r.Action {
//   case setA, createA:
//     srv, err := n.GetServer(sk)
//     if err != nil {
//       return nil, err
//     }
//     return &engine.ServerUpserted{
//       BackendKey: sk.BackendKey,
//       Server:     *srv,
//     }, nil
//   case deleteA, expireA:
//     return &engine.ServerDeleted{
//       ServerKey: sk,
//     }, nil
//   case cswapA: // ignore compare and swap attempts
//     return nil, nil
//   }
//   return nil, fmt.Errorf("unsupported action on the server: %s", r.Action)
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
type cassandraPool struct {
	Name     string
	Keyspace string
	NumConns int
	Nodes    []string
}

type MinimalUserRecord struct {
	Id             gocql.UUID
	Email          string
	HashedPassword string
	PasswordSalt   string
	IsActive       bool
	EncryptionKeys map[string]*string
	MetaStorage    map[string]*string
}

type CustomerRecord struct {
	CustomerId gocql.UUID
	Hostname   string
}
