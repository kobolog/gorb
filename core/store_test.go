package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/docker/libkv"
	libkvmock "github.com/docker/libkv/store/mock"
	"github.com/docker/libkv/store"
)

type storeMock struct {
	libkvmock.Mock
}

func (s *storeMock) mockNew() func(endpoints []string, options *store.Config) (store.Store, error) {
	return func(endpoints []string, options *store.Config) (store.Store, error) {
		s.Endpoints = endpoints
		s.Options = options
		return &s.Mock, nil
	}
}

func TestMultipleURLs(t *testing.T) {
	assert := assert.New(t)
	m := storeMock{}
	libkv.AddStore("mock", m.mockNew())
	m.On("List", "/").Return([]*store.KVPair{}, nil)

	storeURLs := []string{"mock://127.0.0.1:2000", "mock://127.0.0.2:2001", "mock://127.0.0.3:2002"}
	store, err := NewStore(storeURLs, "/", "/", 60, &Context{})

	assert.NoError(err)
	assert.Equal([]string{"127.0.0.1:2000", "127.0.0.2:2001", "127.0.0.3:2002"}, m.Endpoints)

	store.Close()
}

func TestErrorIfSchemeMismatch(t *testing.T) {
	assert := assert.New(t)
	m := storeMock{}
	libkv.AddStore("mock", m.mockNew())
	m.On("List", "/").Return([]*store.KVPair{}, nil)

	storeURLs := []string{"mock://127.0.0.1:2000", "mismatch://127.0.0.2:2001", "mock://127.0.0.3:2002"}
	_, err := NewStore(storeURLs, "/", "/", 60, &Context{})

	assert.Error(err)
}

func TestErrorIfPathMismatch(t *testing.T) {
	assert := assert.New(t)
	m := storeMock{}
	libkv.AddStore("mock", m.mockNew())
	m.On("List", "/").Return([]*store.KVPair{}, nil)

	storeURLs := []string{"mock://127.0.0.1:2000", "mock://127.0.0.2:2001/mismatched/path/", "mock://127.0.0.3:2002"}
	_, err := NewStore(storeURLs, "/", "/", 60, &Context{})

	assert.Error(err)
}
