package core

import (
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
)

type Store struct {
	ctx              *Context
	kvstore          store.Store
	storeServicePath string
	storeBackendPath string
	stopCh           chan struct{}
	locker           *store.Locker
}

func NewStore(storeUrl, storeServicePath, storeBackendPath string, context *Context) (*Store, error) {
	uri, err := url.Parse(storeUrl)
	if err != nil {
		return nil, err
	}
	var backend store.Backend
	switch scheme := strings.ToLower(uri.Scheme); scheme {
	case "consul":
		backend = store.CONSUL
	case "etcd":
		backend = store.ETCD
	case "zookeeper":
		backend = store.ZK
	case "boltdb":
		backend = store.BOLTDB
	default:
		return nil, errors.New("unsupported uri schema : " + uri.Scheme)
	}
	kvstore, err := libkv.NewStore(
		backend,
		[]string{uri.Host},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)

	// create store locker
	locker, err := kvstore.NewLock(
		strings.Trim(path.Join(uri.Path, "gorblock"), "/"),
		&store.LockOptions{Value: []byte("gorb"), TTL: 20 * time.Second},
	)
	if err != nil {
		kvstore.Close()
		return nil, err
	}

	store := &Store{
		ctx:              context,
		kvstore:          kvstore,
		storeServicePath: path.Join(uri.Path, storeServicePath),
		storeBackendPath: path.Join(uri.Path, storeBackendPath),
		stopCh:           make(chan struct{}),
		locker:           &locker,
	}

	return store, nil
}

func (s *Store) StartSync(syncTime int64) {
	s.Sync()
	storeTimer := time.NewTicker(time.Duration(syncTime) * time.Second)
	go func() {
		for {
			select {
			case <-storeTimer.C:
				s.Sync()
			case <-s.stopCh:
				storeTimer.Stop()
				return
			}
		}
	}()
}

func (s *Store) Close() {
	close(s.stopCh)
}

func (s *Store) Sync() {
	if _, err := (*s.locker).Lock(nil); err != nil {
		log.Errorf("error while acquire lock : %s", err)
		return
	}
	// build external services map
	services, err := s.getServices()
	if err != nil {
		log.Errorf("error while get services: %s", err)
		(*s.locker).Unlock()
		return
	}
	// build external backends map
	backends, err := s.getBackends()
	if err != nil {
		log.Errorf("error while get backends: %s", err)
		(*s.locker).Unlock()
		return
	}
	(*s.locker).Unlock()
	// synchronize context
	s.ctx.Synchronize(services, backends)
}

func (s *Store) GetServices() (map[string]*ServiceOptions, error) {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return nil, err
	}
	defer (*s.locker).Unlock()
	return s.getServices()
}

func (s *Store) getServices() (map[string]*ServiceOptions, error) {
	services := make(map[string]*ServiceOptions)
	// build external service map (temporary all services)
	kvlist, err := s.kvstore.List(s.storeServicePath)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return services, nil
		}
		return nil, err
	}
	for _, kvpair := range kvlist {
		id := s.getID(kvpair.Key)
		var options ServiceOptions
		if err := json.Unmarshal(kvpair.Value, &options); err != nil {
			return nil, err
		}
		services[id] = &options
	}
	return services, nil
}

func (s *Store) GetBackends() (map[string]*BackendOptions, error) {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return nil, err
	}
	defer (*s.locker).Unlock()
	return s.getBackends()
}

func (s *Store) getBackends() (map[string]*BackendOptions, error) {
	backends := make(map[string]*BackendOptions)
	// build external backend map
	kvlist, err := s.kvstore.List(s.storeBackendPath)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return backends, nil
		}
		return nil, err
	}
	for _, kvpair := range kvlist {
		var options BackendOptions
		if err := json.Unmarshal(kvpair.Value, &options); err != nil {
			return nil, err
		}
		backends[s.getID(kvpair.Key)] = &options
	}
	return backends, nil
}

func (s *Store) CreateService(vsID string, opts *ServiceOptions) error {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return err
	}
	defer (*s.locker).Unlock()
	// put to store
	if err := s.put(s.storeServicePath+"/"+vsID, opts, false); err != nil {
		log.Errorf("error while put service to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) CreateBackend(vsID, rsID string, opts *BackendOptions) error {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return err
	}
	defer (*s.locker).Unlock()
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath+"/"+rsID, opts, false); err != nil {
		log.Errorf("error while put backend to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) UpdateBackend(vsID, rsID string, opts *BackendOptions) error {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return err
	}
	defer (*s.locker).Unlock()
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath+"/"+rsID, opts, true); err != nil {
		log.Errorf("error while put(update) backend to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) RemoveService(vsID string) error {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return err
	}
	defer (*s.locker).Unlock()
	// remove from store
	if err := s.kvstore.DeleteTree(s.storeServicePath + "/" + vsID); err != nil {
		log.Errorf("error while delete service from store: %s", err)
		return err
	}
	return nil
}

func (s *Store) RemoveBackend(rsID string) error {
	if _, err := (*s.locker).Lock(nil); err != nil {
		return err
	}
	defer (*s.locker).Unlock()
	// remove from store
	if err := s.kvstore.DeleteTree(s.storeBackendPath + "/" + rsID); err != nil {
		log.Errorf("error while delete backend from store: %s", err)
		return err
	}
	return nil
}

func (s *Store) put(key string, value interface{}, overwrite bool) error {
	// marshal value
	var byteValue []byte
	var isDir bool
	if value == nil {
		byteValue = nil
		isDir = true
	} else {
		_bytes, err := json.Marshal(value)
		if err != nil {
			return err
		}
		byteValue = _bytes
		isDir = false
	}
	// check key exist (create if not exists)
	exist, err := s.kvstore.Exists(key)
	if err != nil {
		return err
	}
	if !exist || overwrite {
		writeOptions := &store.WriteOptions{IsDir: isDir, TTL: 0}
		if err := s.kvstore.Put(key, byteValue, writeOptions); err != nil {
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

func init() {
	consul.Register()
	etcd.Register()
	zookeeper.Register()
	boltdb.Register()
}
