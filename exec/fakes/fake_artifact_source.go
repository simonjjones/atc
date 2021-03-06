// This file was generated by counterfeiter
package fakes

import (
	"io"
	"sync"

	"github.com/concourse/atc/exec"
)

type FakeArtifactSource struct {
	StreamToStub        func(exec.ArtifactDestination) error
	streamToMutex       sync.RWMutex
	streamToArgsForCall []struct {
		arg1 exec.ArtifactDestination
	}
	streamToReturns struct {
		result1 error
	}
	StreamFileStub        func(path string) (io.ReadCloser, error)
	streamFileMutex       sync.RWMutex
	streamFileArgsForCall []struct {
		path string
	}
	streamFileReturns struct {
		result1 io.ReadCloser
		result2 error
	}
}

func (fake *FakeArtifactSource) StreamTo(arg1 exec.ArtifactDestination) error {
	fake.streamToMutex.Lock()
	fake.streamToArgsForCall = append(fake.streamToArgsForCall, struct {
		arg1 exec.ArtifactDestination
	}{arg1})
	fake.streamToMutex.Unlock()
	if fake.StreamToStub != nil {
		return fake.StreamToStub(arg1)
	} else {
		return fake.streamToReturns.result1
	}
}

func (fake *FakeArtifactSource) StreamToCallCount() int {
	fake.streamToMutex.RLock()
	defer fake.streamToMutex.RUnlock()
	return len(fake.streamToArgsForCall)
}

func (fake *FakeArtifactSource) StreamToArgsForCall(i int) exec.ArtifactDestination {
	fake.streamToMutex.RLock()
	defer fake.streamToMutex.RUnlock()
	return fake.streamToArgsForCall[i].arg1
}

func (fake *FakeArtifactSource) StreamToReturns(result1 error) {
	fake.StreamToStub = nil
	fake.streamToReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeArtifactSource) StreamFile(path string) (io.ReadCloser, error) {
	fake.streamFileMutex.Lock()
	fake.streamFileArgsForCall = append(fake.streamFileArgsForCall, struct {
		path string
	}{path})
	fake.streamFileMutex.Unlock()
	if fake.StreamFileStub != nil {
		return fake.StreamFileStub(path)
	} else {
		return fake.streamFileReturns.result1, fake.streamFileReturns.result2
	}
}

func (fake *FakeArtifactSource) StreamFileCallCount() int {
	fake.streamFileMutex.RLock()
	defer fake.streamFileMutex.RUnlock()
	return len(fake.streamFileArgsForCall)
}

func (fake *FakeArtifactSource) StreamFileArgsForCall(i int) string {
	fake.streamFileMutex.RLock()
	defer fake.streamFileMutex.RUnlock()
	return fake.streamFileArgsForCall[i].path
}

func (fake *FakeArtifactSource) StreamFileReturns(result1 io.ReadCloser, result2 error) {
	fake.StreamFileStub = nil
	fake.streamFileReturns = struct {
		result1 io.ReadCloser
		result2 error
	}{result1, result2}
}

var _ exec.ArtifactSource = new(FakeArtifactSource)
