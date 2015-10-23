package vulcan

import (
	"encoding/json"
	"fmt"

	"github.com/fkasper/core/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

const (
	backendPath  = "vulcand/backends/%v"
	serverPath   = "vulcand/backends/%v/servers/%v"
	frontendPath = "vulcand/frontends/%v.%v"

	endpointTTL = 5 // seconds
)

type Registry struct {
	etcdClient *etcd.Client
	config     Config
}

type Config struct {
	PublicAPIHost    string
	ProtectedAPIHost string
	Hosts            []string
}

func NewRegistry(config Config, hosts []string) *Registry {
	return &Registry{
		etcdClient: etcd.NewClient(hosts),
		config:     config,
	}
}

func (r *Registry) RegisterBackend(e *Endpoint) error {
	bk := fmt.Sprintf(backendPath+"/backend", e.Name)
	bkSpec, err := e.BackendSpec()
	if err != nil {
		return err
	}
	_, err = r.etcdClient.Set(bk, bkSpec, 0)
	return err
}

func (r *Registry) RegisterServer(e *Endpoint) error {
	if err := r.RegisterBackend(e); err != nil {
		return err
	}
	sSpec, err := e.ServerSpec()
	if err != nil {
		return err
	}
	sk := fmt.Sprintf(serverPath, e.Name, e.ID)
	if _, err := r.etcdClient.Set(sk, sSpec, endpointTTL); err != nil {
		return err
	}

	return nil
}

func (r *Registry) RegisterFrontend(l *Location) error {
	fk := fmt.Sprintf(frontendPath, l.Host, l.ID)

	spec, err := l.Spec()
	if err != nil {
		return err
	}

	if _, err := r.etcdClient.Set(fk+"/frontend", spec, 0); err != nil {
		return err
	}

	for i, m := range l.Middlewares {
		// each middleware has a priority defining its order in the execution chain;
		// assign them priorities according to their positions in the list
		m.Priority = i

		middleware, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("failed to marshal %v: %v", m, err)
		}

		middlewareKey := fmt.Sprintf("%v/middlewares/%v", fk, m.ID)
		if _, err := r.etcdClient.Set(middlewareKey, string(middleware), 0); err != nil {
			return err
		}
	}

	return nil
}
