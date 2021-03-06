package pipelines

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/factory"
)

//go:generate counterfeiter . Locker

type Locker interface {
	AcquireWriteLock([]db.NamedLock) (db.Lock, error)
	AcquireWriteLockImmediately([]db.NamedLock) (db.Lock, error)

	AcquireReadLock([]db.NamedLock) (db.Lock, error)
}

//go:generate counterfeiter . RadarSchedulerFactory

type RadarSchedulerFactory interface {
	BuildRadar(pipelineDB db.PipelineDB) *radar.Radar
	BuildScheduler(pipelineDB db.PipelineDB) *scheduler.Scheduler
}

type radarSchedulerFactory struct {
	tracker  resource.Tracker
	interval time.Duration
	locker   Locker
	engine   engine.Engine
	db       db.DB
}

func NewRadarSchedulerFactory(
	tracker resource.Tracker,
	interval time.Duration,
	locker Locker,
	engine engine.Engine,
	db db.DB,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		tracker:  tracker,
		interval: interval,
		locker:   locker,
		engine:   engine,
		db:       db,
	}
}

func (rsf *radarSchedulerFactory) BuildRadar(pipelineDB db.PipelineDB) *radar.Radar {
	return radar.NewRadar(rsf.tracker, rsf.interval, rsf.locker, pipelineDB)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipelineDB db.PipelineDB) *scheduler.Scheduler {
	radar := rsf.BuildRadar(pipelineDB)
	return &scheduler.Scheduler{
		PipelineDB: pipelineDB,
		BuildsDB:   rsf.db,
		Factory:    &factory.BuildFactory{PipelineName: pipelineDB.GetPipelineName()},
		Engine:     rsf.engine,
		Scanner:    radar,
	}
}
