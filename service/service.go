package service

import (
	//"encoding/json"
	"fmt"
	"io/ioutil"
	//"net"
	"net/http"
	"os"
	//"os/exec"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/scroll"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/rs/cors"
	"os/signal"
	"syscall"
	"time"
	//"github.com/mailgun/scroll/registry"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/manners"
	"github.com/fkasper/core/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/fkasper/core/api"
	"github.com/fkasper/core/engine"
	"github.com/fkasper/core/engine/etcdng"
	"github.com/fkasper/core/vulcan"
	"github.com/fkasper/core/vulcan/middleware"
)

func Run() error {
	options, err := ParseCommandLine()
	if err != nil {
		return fmt.Errorf("failed to parse command line: %s", err)
	}
	service := NewService(options)
	if err := service.Start(); err != nil {
		log.Errorf("Failed to start service: %v", err)
		return fmt.Errorf("service start failure: %s", err)
	} else {
		log.Infof("Service exited gracefully")
	}
	return nil
}

type Service struct {
	client        *etcd.Client
	options       Options
	apiApp        *scroll.App
	errorC        chan error
	sigC          chan os.Signal
	metricsClient metrics.Client
	apiServer     *manners.GracefulServer
	ng            engine.Engine
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		errorC:  make(chan error),
		// Channel receiving signals has to be non blocking, otherwise the service can miss a signal.
		sigC: make(chan os.Signal, 1024),
	}
}

func (s *Service) Start() error {
	log.InitWithConfig(log.Config{
		Name:     s.options.Log,
		Severity: s.options.LogSeverity.S.String(),
	})

	log.Infof("Service starts with options: %#v", s.options)

	if s.options.PidPath != "" {
		ioutil.WriteFile(s.options.PidPath, []byte(fmt.Sprint(os.Getpid())), 0644)
	}

	if s.options.StatsdAddr != "" {
		var err error
		s.metricsClient, err = metrics.NewWithOptions(s.options.StatsdAddr, s.options.StatsdPrefix, metrics.Options{UseBuffering: true})
		if err != nil {
			return err
		}
	}

	// apiFile, muxFiles, err := s.getFiles()
	// if err != nil {
	// 	return err
	// }

	// if err := s.newEngine(); err != nil {
	// 	return err
	// }

	// s.stapler = stapler.New()
	// s.supervisor = supervisor.New(
	// 	s.newProxy, s.ng, s.errorC, supervisor.Options{Files: muxFiles})

	// Tells configurator to perform initial proxy configuration and start watching changes
	// if err := s.supervisor.Start(); err != nil {
	// 	return err
	// }
	if err := s.newEngine(); err != nil {
		return err
	}

	if err := s.initApi(); err != nil {
		return err
	}

	go func() {
		s.errorC <- s.startApi()
	}()

	if s.metricsClient != nil {
		go s.reportSystemMetrics()
	}
	signal.Notify(s.sigC, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGUSR2, syscall.SIGCHLD)

	// Block until a signal is received or we got an error
	for {
		select {
		case signal := <-s.sigC:
			switch signal {
			case syscall.SIGTERM, syscall.SIGINT:
				log.Infof("Got signal '%s', shutting down gracefully", signal)
				//s.supervisor.Stop(true)
				log.Infof("All servers stopped")
				return nil
			case syscall.SIGKILL:
				log.Infof("Got signal '%s', exiting now without waiting", signal)
				//s.supervisor.Stop(false)
				return nil
			default:
				log.Infof("Ignoring '%s'", signal)
			}
		case err := <-s.errorC:
			log.Infof("Got request to shutdown with error: %s", err)
			return err
		}
	}
}

func (s *Service) reportSystemMetrics() {
	defer func() {
		if r := recover(); r != nil {
			log.Infof("Recovered in reportSystemMetrics", r)
		}
	}()
	for {
		s.metricsClient.ReportRuntimeMetrics("sys", 1.0)
		// we have 256 time buckets for gc stats, GC is being executed every 4ms on average
		// so we have 256 * 4 = 1024 around one second to report it. To play safe, let's report every 300ms
		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Service) initApi() error {
	s.apiApp = scroll.NewApp()
	api.InitProxyController(s.ng, s.apiApp)
	return nil
}

func (s *Service) startApi() error {
	addr := fmt.Sprintf("%s:%d", s.options.Interface, s.options.Port)
	handler := cors.Default().Handler(s.apiApp.GetHandler())

	//go func() {

	//}()

	server := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    s.options.ServerReadTimeout,
		WriteTimeout:   s.options.ServerWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	s.apiServer = manners.NewWithServer(server)
	return s.apiServer.ListenAndServe()
}

func (s *Service) newEngine() error {
	ng, err := etcdng.New(
		s.options.EtcdNodes,
		s.options.EtcdKey,
		etcdng.Options{
			EtcdCaFile:      s.options.EtcdCaFile,
			EtcdCertFile:    s.options.EtcdCertFile,
			EtcdKeyFile:     s.options.EtcdKeyFile,
			EtcdConsistency: s.options.EtcdConsistency,
		})
	if err != nil {
		return err
	}
	s.ng = ng

	config := vulcan.Config{
		PublicAPIHost:    "http://127.0.0.1:8182",
		ProtectedAPIHost: "http://127.0.0.1:8182",
	}
	reg := vulcan.NewRegistry(config, s.options.EtcdNodes)
	entry, _ := vulcan.NewEndpointWithID(s.options.ApplicationId, s.options.ApplicationId, s.options.Interface, s.options.Port)
	if err := reg.RegisterBackend(entry); err != nil {
		return err
	}

	if err := reg.RegisterServer(entry); err != nil {
		return err
	}

	//host string, methods []string, path, upstream string, middlewares []middleware.Middleware
	loc := vulcan.NewLocation(s.options.HostLimit, []string{}, s.options.RegisterPath, s.options.ApplicationId,
		[]middleware.Middleware{})
	if err := reg.RegisterFrontend(loc); err != nil {
		return err
	}

	return err
}
