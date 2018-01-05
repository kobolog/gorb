package core

import (
	"errors"
	"net/url"
	"path"
	"strings"
	"time"

	"encoding/json"
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
}

func NewStore(storeURLs []string, storeServicePath, storeBackendPath string, syncTime int64, context *Context) (*Store, error) {
	var scheme string
	var storePath string
	var hosts []string
	for _, storeURL := range storeURLs {
		uri, err := url.Parse(storeURL)
		if err != nil {
			return nil, err
		}
		uriScheme := strings.ToLower(uri.Scheme)
		if scheme != "" && scheme != uriScheme {
			return nil, errors.New("schemes must be the same for all store URLs")
		}
		if storePath != "" && storePath != uri.Path {
			return nil, errors.New("paths must be the same for all store URLs")
		}
		scheme = uriScheme
		storePath = uri.Path
		hosts = append(hosts, uri.Host)
	}

	var backend store.Backend
	switch scheme {
	case "consul":
		backend = store.CONSUL
	case "etcd":
		backend = store.ETCD
	case "zookeeper":
		backend = store.ZK
	case "boltdb":
		backend = store.BOLTDB
	case "mock":
		backend = "mock"
	default:
		return nil, errors.New("unsupported uri scheme : " + scheme)
	}
	kvstore, err := libkv.NewStore(
		backend,
		hosts,
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	if err != nil {
		return nil, err
	}

	store := &Store{
		ctx:              context,
		kvstore:          kvstore,
		storeServicePath: path.Join(storePath, storeServicePath),
		storeBackendPath: path.Join(storePath, storeBackendPath),
		stopCh:           make(chan struct{}),
	}

	context.SetStore(store)

	store.Sync()
	storeTimer := time.NewTicker(time.Duration(syncTime) * time.Second)
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
	// build external services map
	services, err := s.getExternalServices()
	if err != nil {
		log.Errorf("error while get services: %s", err)
		return
	}
	// build external backends map
	backends, err := s.getExternalBackends()
	if err != nil {
		log.Errorf("error while get backends: %s", err)
		return
	}
	// synchronize context
	s.ctx.Synchronize(services, backends)
}

func (s *Store) getExternalServices() (map[string]*ServiceOptions, error) {
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

func (s *Store) getExternalBackends() (map[string]*BackendOptions, error) {
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

func (s *Store) Close() {
	close(s.stopCh)
}

func (s *Store) CreateService(vsID string, opts *ServiceOptions) error {
	// put to store
	if err := s.put(s.storeServicePath+"/"+vsID, opts, false); err != nil {
		log.Errorf("error while put service to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) CreateBackend(vsID, rsID string, opts *BackendOptions) error {
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath+"/"+rsID, opts, false); err != nil {
		log.Errorf("error while put backend to store: %s", err)
		return err
	}
	return nil
}

func (s *Store) UpdateBackend(vsID, rsID string, opts *BackendOptions) error {
	opts.VsID = vsID
	// put to store
	if err := s.put(s.storeBackendPath+"/"+rsID, opts, true); err != nil {
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
