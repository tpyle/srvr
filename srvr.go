package srvr

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	logmanager "github.com/tpyle/log-manager/v2"
	logmiddleware "github.com/tpyle/log-middleware/v2"
)

type Server struct {
	options        *Options
	internalServer *http.Server
	externalServer *http.Server
	waitGroup      *sync.WaitGroup
}

func (s *Server) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	for name, check := range s.options.readinessChecks {
		if err := check(r.Context()); err != nil {
			log.Warn().Err(err).Msgf("Readiness check '%s' failed", name)
			http.Error(w, fmt.Sprintf("Readiness check '%s' failed: %v", name, err), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func Create(opts ...Option) *Server {
	o := defaultOptions()

	for _, opt := range opts {
		opt(o)
	}

	server := &Server{
		options:   o,
		waitGroup: &sync.WaitGroup{},
	}

	if o.internalRouter != nil {
		if o.doLogInternalRouter {
			o.internalRouter.Use(logmiddleware.NewLogMiddleware(&log.Logger).Handler)
		}

		if o.logManager != nil {
			o.internalRouter.HandleFunc(o.logManager.Path, logmanager.HandleLogCall).Methods("POST", "GET")
		}
		if o.metricsManager != nil {
			o.internalRouter.Path(o.metricsManager.Path).Handler(promhttp.Handler())
		}
		if o.livenessManager != nil {
			o.internalRouter.HandleFunc(o.livenessManager.Path, server.livenessHandler).Methods("GET")
		}
		if o.readinessManager != nil {
			o.internalRouter.HandleFunc(o.readinessManager.Path, server.readinessHandler).Methods("GET")
		}

		server.internalServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", o.internalPort),
			Handler:           o.internalRouter,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ReadHeaderTimeout: o.readHeaderTimeout,
			MaxHeaderBytes:    o.maxHeaderBytes,
		}
	}

	if o.externalRouter != nil {
		if o.doLogExternalRouter {
			o.externalRouter.Use(logmiddleware.NewLogMiddleware(&log.Logger).Handler)
		}

		if o.metricsManager != nil {
			o.externalRouter.Use(func(next http.Handler) http.Handler {
				return promhttp.InstrumentMetricHandler(
					prometheus.DefaultRegisterer, next,
				)
			})
		}

		server.externalServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", o.externalPort),
			Handler:           o.externalRouter,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ReadHeaderTimeout: o.readHeaderTimeout,
			MaxHeaderBytes:    o.maxHeaderBytes,
		}
	}

	return server
}

func (s *Server) Start() error {
	if s.internalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Internal server error")
			}
		})
		log.Info().Msgf("Internal server started on port %d", s.options.internalPort)
	}

	if s.externalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.externalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("External server error")
			}
		})
		log.Info().Msgf("External server started on port %d", s.options.externalPort)
	}

	return nil
}

// Comparable to Start, but blocks the current goroutine
func (s *Server) Run() {
	if s.internalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Internal server error")
			}
		})
		log.Info().Msgf("Internal server started on port %d", s.options.internalPort)
	}

	if s.externalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.externalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("External server error")
			}
		})
		log.Info().Msgf("External server started on port %d", s.options.externalPort)
	}

	s.waitGroup.Wait()
}

func (s *Server) Wait() {
	s.waitGroup.Wait()
}

func (s *Server) Stop() error {
	if s.internalServer != nil {
		if err := s.internalServer.Close(); err != nil {
			return err
		}
		log.Info().Msg("Internal server stopped")
	}

	if s.externalServer != nil {
		if err := s.externalServer.Close(); err != nil {
			return err
		}
		log.Info().Msg("External server stopped")
	}

	return nil
}
