// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/getjoblessbuild"
)

type FakeBuildDB struct {
	GetBuildStub        func(int) (db.Build, error)
	getBuildMutex       sync.RWMutex
	getBuildArgsForCall []struct {
		arg1 int
	}
	getBuildReturns struct {
		result1 db.Build
		result2 error
	}
}

func (fake *FakeBuildDB) GetBuild(arg1 int) (db.Build, error) {
	fake.getBuildMutex.Lock()
	fake.getBuildArgsForCall = append(fake.getBuildArgsForCall, struct {
		arg1 int
	}{arg1})
	fake.getBuildMutex.Unlock()
	if fake.GetBuildStub != nil {
		return fake.GetBuildStub(arg1)
	} else {
		return fake.getBuildReturns.result1, fake.getBuildReturns.result2
	}
}

func (fake *FakeBuildDB) GetBuildCallCount() int {
	fake.getBuildMutex.RLock()
	defer fake.getBuildMutex.RUnlock()
	return len(fake.getBuildArgsForCall)
}

func (fake *FakeBuildDB) GetBuildArgsForCall(i int) int {
	fake.getBuildMutex.RLock()
	defer fake.getBuildMutex.RUnlock()
	return fake.getBuildArgsForCall[i].arg1
}

func (fake *FakeBuildDB) GetBuildReturns(result1 db.Build, result2 error) {
	fake.GetBuildStub = nil
	fake.getBuildReturns = struct {
		result1 db.Build
		result2 error
	}{result1, result2}
}

var _ getjoblessbuild.BuildDB = new(FakeBuildDB)
