package etcd

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/ti/noframe/config"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

type etcdBackend struct {
	url      *url.URL
	instance interface{}
	onLoaded config.OnLoaded
}

// New new instance
func New() *etcdBackend {
	return &etcdBackend{}
}

var client *clientv3.Client

//GetEtcd get ETCD client
func GetEtcd() *clientv3.Client {
	return client
}

func init() {
	config.AddBackend("etcd", &etcdBackend{})
}

// LoadConfig gets the JSON from ETCD and unmarshals it to the config object
func (e *etcdBackend) LoadConfig(o config.Options) error {
	if o.DefaultConfig == nil {
		//this should not be happen
		panic("default config can not be nil")
	}
	var err error
	var newEtcd bool
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if e.url == nil {
		u, err := url.Parse(o.URL)
		if err != nil {
			return err
		}
		e.url = u
		e.instance = o.DefaultConfig
		e.onLoaded = o.OnLoaded
	}
	if client == nil {
		//first time to load config
		client, err = newEtcdClient(e.url)
		if err != nil {
			return err
		}
		newEtcd = true
	}
	prefixKeys := config.GetPrefixKeys(e.url.Path, o.DefaultConfig)
	etcdKvs, err := e.getKvs(ctx, prefixKeys)
	if err != nil {
		if !newEtcd {
			client.Close()
			client = nil
			log.Warnf("etcd get key error ", err, " try 1 time")
			client, err = newEtcdClient(e.url)
			if err != nil {
				return err
			}
			newEtcd = true
			etcdKvs, err = e.getKvs(ctx, prefixKeys)
		}
	}
	if err != nil {
		return fmt.Errorf("bad cluster endpoints, which are not etcd servers: %v", err)
	}

	if len(etcdKvs) == 0 {
		kvs, err := config.Marshal(e.url.Path, e.instance)
		if err != nil {
			return fmt.Errorf("path %s marshal error %s", e.url.Path, err)
		}
		for _, kv := range kvs {
			if _, err := client.Put(ctx, kv.Key, kv.Value); err != nil {
				return fmt.Errorf("key not found: %s, put error %s", e.url.Path, err)
			}
		}
	} else {
		var kvs []*config.KV
		for _, kv := range etcdKvs {
			kvs = append(kvs, &config.KV{
				Key:   string(kv.Key),
				Value: string(kv.Value),
			})
		}
		err = config.Unmarshal(e.url.Path, kvs, e.instance)
		if err != nil {
			return err
		}
	}
	watch := o.Watch && e.onLoaded != nil
	if newEtcd && watch {
		go e.watch(context.Background(), e.url.Path, prefixKeys)
	}
	if !watch {
		client.Close()
		client = nil
	}
	e.onLoaded(e.instance)
	return nil
}

func (e *etcdBackend) getKvs(ctx context.Context, keys []string) (kvs []*mvccpb.KeyValue, err error) {
	for _, key := range keys {
		var opts []clientv3.OpOption
		if strings.HasSuffix(key, "/") {
			opts = append(opts, clientv3.WithPrefix())
		}
		getResp, err := client.Get(ctx, key, opts...)
		if err != nil {
			return nil, err
		}
		if len(getResp.Kvs) > 0 {
			kvs = append(kvs, getResp.Kvs...)
		}
	}
	return
}

func (e *etcdBackend) watch(ctx context.Context, rootKey string, keys []string) {
	var watchKeys []string
	var watchRootKeys []string
	for _, k := range keys {
		if strings.HasPrefix(k, rootKey) {
			watchRootKeys = append(watchRootKeys, k)
		} else {
			watchKeys = append(watchKeys, k)
		}
	}
	for _, key := range watchKeys {
		var opts []clientv3.OpOption
		if strings.HasSuffix(key, "/") {
			opts = append(opts, clientv3.WithPrefix())
		}
		wc := client.Watch(ctx, key, opts...)
		go e.onEtcdWatch(ctx, keys, wc)
	}
	var opts []clientv3.OpOption
	if len(watchRootKeys) > 1 {
		opts = append(opts, clientv3.WithPrefix())
	}
	wc := client.Watch(ctx, e.url.Path, opts...)
	e.onEtcdWatch(ctx, keys, wc)
}

func (e *etcdBackend) onEtcdWatch(ctx context.Context, keys []string, wc clientv3.WatchChan) {
	for wresp := range wc {
		if wresp.Err() != nil {
			log.Errorf("Watch channel returned err %v", wresp.Err())
			return
		}
		var isChange bool
		for _, ev := range wresp.Events {
			if ev.Type == clientv3.EventTypePut || ev.Type == clientv3.EventTypeDelete {
				isChange = true
			}
		}
		if isChange {
			etcdKvs, err := e.getKvs(ctx, keys)
			if err != nil {
				log.Errorf("Watch channel get prefix err %v", err)
				continue
			}
			var kvs []*config.KV
			for _, kv := range etcdKvs {
				kvs = append(kvs, &config.KV{
					Key:   string(kv.Key),
					Value: string(kv.Value),
				})
			}
			if err := config.Unmarshal(e.url.Path, kvs, e.instance); err != nil {
				log.Errorf("Watch channel unmarshal err %s", err.Error())
				continue
			} else {
				e.onLoaded(e.instance)
			}
		}
	}
}

func newEtcdClient(etcdUri *url.URL) (*clientv3.Client, error) {
	etcdConfig := clientv3.Config{
		Endpoints:   strings.Split(etcdUri.Host, ","),
		DialTimeout: 30 * time.Second,
	}
	if etcdUri.User != nil && etcdUri.User.Username() != "" {
		etcdConfig.Username = etcdUri.User.Username()
		etcdConfig.Password, _ = etcdUri.User.Password()
	}
	etcdUriQuery := etcdUri.Query()
	cert := etcdUriQuery.Get("cert")
	if cert != "" {
		key := etcdUriQuery.Get("key")
		ca := etcdUriQuery.Get("ca")
		// Load client cert
		tlsInfo := transport.TLSInfo{
			CertFile:      cert,
			KeyFile:       key,
			TrustedCAFile: ca,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		// Add TLS config
		etcdConfig.TLS = tlsConfig
	}
	return clientv3.New(etcdConfig)
}
