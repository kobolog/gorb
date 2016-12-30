package core

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
	log "github.com/Sirupsen/logrus"
	"encoding/json"
)

func init() {
	consul.Register()
	etcd.Register()
	zookeeper.Register()
	boltdb.Register()
}

type Store struct {
	context          *Context
	kvstore          store.Store
	storeServicePath string
	storeBackendPath string
	stopCh  chan struct{}
}

func NewStore(storeUrl, storeServicePath, storeBackendPath string, context *Context) (*Store, error) {
	uri, err := url.Parse(storeUrl)
	if err != nil {
		return nil, err
	}
	var backend store.Backend
	if strings.EqualFold(uri.Scheme, "consul") {
		backend = store.CONSUL
	} else if strings.EqualFold(uri.Scheme, "etcd") {
		backend = store.ETCD
	} else if strings.EqualFold(uri.Scheme, "zookeeper") {
		backend = store.ZK
	} else if strings.EqualFold(uri.Scheme, "boltdb") {
		backend = store.BOLTDB
	} else {
		return nil, errors.New("unsupported uri schema : " + uri.Scheme)
	}
	kvstore, err := libkv.NewStore(
		backend,
		[]string{ uri.Host },
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)

	store := &Store{
		context:          context,
		kvstore:          kvstore,
		storeServicePath: storeServicePath,
		storeBackendPath: storeBackendPath,
		stopCh:           make(chan struct{}),
	}

	context.SetStore(store)

	store.Sync()
	storeTimer := time.NewTicker(time.Duration(3) * time.Second)
	go func() {
		for {
			select {
			case <-storeTimer.C:
				store.Sync()
			case <-store.stopCh:
				storeTimer.Stop()
				return
			}
		}
	}()

	return store, nil
}

func (s *Store) Sync() {
	// build external backends map
	backends, err := s.getExternalBackends()
	if err != nil {
		log.Errorf("error while get backends: %s", err)
		return
	}
	// build external services map
	services, err := s.getExternalServices(backends)
	if err != nil {
		log.Errorf("error while get services: %s", err)
		return
	}
	log.Info("============================== SYNC ========================================")
	for k, v := range services {
		log.Info("SERVICE[%s]: %s", k, v)
	}
	for k, v := range backends {
		log.Info("  BACKEND[%s]: %s", k, v)
	}
	log.Info("============================================================================")
}

func (s *Store) getExternalBackends() (map[string]*BackendOptions, error) {
	// build external backend map
	kvlist, err := s.kvstore.List(s.storeBackendPath)
	if err != nil {
		return nil, err
	}
	backends := make(map[string]*BackendOptions)
	for _, kvpair := range kvlist {
		var options BackendOptions
		if err := json.Unmarshal(kvpair.Value, &options); err != nil {
			return nil, err
		}
		backends[s.getID(kvpair.Key)] = &options
	}
	return backends, nil
}

func (s *Store) getExternalServices(backends map[string]*BackendOptions) (map[string]*ServiceOptions, error) {
	// build services id map for filter services which doesn't have any backends
	filter := make(map[string]bool)
	for _, backendOption := range backends {
		filter[backendOption.VsID] = true
	}
	services := make(map[string]*ServiceOptions)
	// build external service map (temporary all services)
	kvlist, err := s.kvstore.List(s.storeServicePath)
	if err != nil {
		return nil, err
	}
	for _, kvpair := range kvlist {
		id := s.getID(kvpair.Key)
		if _, ok := filter[id]; !ok {
			continue
		}
		var options ServiceOptions
		if err := json.Unmarshal(kvpair.Value, &options); err != nil {
			return nil, err
		}
		services[id] = &options
	}
	return services, nil
}

func (s *Store) Close() {
	close(s.stopCh)
}

func (s *Store) CreateService(vsID string, opts *ServiceOptions) error {
	// put to store
	if err := s.put(s.storeServicePath + "/" + vsID, opts, false); err != nil {
		log.Errorf("error while put service to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) CreateBackend(vsID, rsID string, opts *BackendOptions) error {
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath + "/" + rsID, opts, false); err != nil {
		log.Errorf("error while put backend to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) UpdateBackend(vsID, rsID string, opts *BackendOptions) error {
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath + "/" + rsID, opts, true); err != nil {
		log.Errorf("error while put(update) backend to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) RemoveService(vsID string) error {
	if err := s.kvstore.DeleteTree(s.storeServicePath + "/" + vsID); err != nil {
		log.Errorf("error while delete service from store: %s", err)
		return err
	}
	return nil
}

func (s *Store) RemoveBackend(rsID string) error {
	if err := s.kvstore.DeleteTree(s.storeBackendPath + "/" + rsID); err != nil {
		log.Errorf("error while delete backend from store: %s", err)
		return err
	}
	return nil
}

func (s *Store) put(key string, value interface{}, overwrite bool) error {
	// marshal value
	var _value []byte
	var _IsDir bool
	if value == nil {
		_value = nil
		_IsDir = true
	} else {
		_bytes, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_value = _bytes
		_IsDir = false
	}
	// check key exist (create if not exists)
	exist, err := s.kvstore.Exists(key)
	if err != nil {
		return err
	}
	if !exist || overwrite {
		writeOptions := &store.WriteOptions{ IsDir:_IsDir, TTL:0 }
		if err := s.kvstore.Put(key, _value, writeOptions); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) getID(key string) string {
	index := strings.LastIndex(key, "/")
	if index <= 0 {
		return key
	}
	return key[index+1:]
}