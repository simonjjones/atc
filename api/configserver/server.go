package configserver

import (
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	db           db.ConfigDB
	validate     ConfigValidator
	workerClient worker.Client
}

type ConfigValidator func(atc.Config) error

func NewServer(
	logger lager.Logger,
	db db.ConfigDB,
	validator ConfigValidator,
	workerClient worker.Client,
) *Server {
	return &Server{
		logger:       logger,
		db:           db,
		validate:     validator,
		workerClient: workerClient,
	}
}
