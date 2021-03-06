// This file was generated by counterfeiter
package fakes

import (
	"net/http"
	"sync"

	"github.com/concourse/atc/auth"
)

type FakeValidator struct {
	IsAuthenticatedStub        func(*http.Request) bool
	isAuthenticatedMutex       sync.RWMutex
	isAuthenticatedArgsForCall []struct {
		arg1 *http.Request
	}
	isAuthenticatedReturns struct {
		result1 bool
	}
}

func (fake *FakeValidator) IsAuthenticated(arg1 *http.Request) bool {
	fake.isAuthenticatedMutex.Lock()
	fake.isAuthenticatedArgsForCall = append(fake.isAuthenticatedArgsForCall, struct {
		arg1 *http.Request
	}{arg1})
	fake.isAuthenticatedMutex.Unlock()
	if fake.IsAuthenticatedStub != nil {
		return fake.IsAuthenticatedStub(arg1)
	} else {
		return fake.isAuthenticatedReturns.result1
	}
}

func (fake *FakeValidator) IsAuthenticatedCallCount() int {
	fake.isAuthenticatedMutex.RLock()
	defer fake.isAuthenticatedMutex.RUnlock()
	return len(fake.isAuthenticatedArgsForCall)
}

func (fake *FakeValidator) IsAuthenticatedArgsForCall(i int) *http.Request {
	fake.isAuthenticatedMutex.RLock()
	defer fake.isAuthenticatedMutex.RUnlock()
	return fake.isAuthenticatedArgsForCall[i].arg1
}

func (fake *FakeValidator) IsAuthenticatedReturns(result1 bool) {
	fake.IsAuthenticatedStub = nil
	fake.isAuthenticatedReturns = struct {
		result1 bool
	}{result1}
}

var _ auth.Validator = new(FakeValidator)
