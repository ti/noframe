package etcd

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	log "github.com/sirupsen/logrus"
	"github.com/ti/noframe/config"
)

type etcdBackend struct {
	url      *url.URL
	client   *clientv3.Client
	instance interface{}
	onLoaded config.OnLoaded
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
	var getResp *clientv3.GetResponse
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
	if e.client == nil {
		//first time to load config
		e.client, err = newEtcdClient(e.url)
		if err != nil {
			return err
		}
		getResp, err = e.client.Get(ctx, e.url.Path, clientv3.WithPrefix())
		newEtcd = true
	} else {
		getResp, err = e.client.Get(ctx, e.url.Path, clientv3.WithPrefix())
		if err != nil {
			e.client.Close()
			e.client = nil
			log.Warnf("etcd get key error ", err, " try 1 time")
			e.client, err = newEtcdClient(e.url)
			if err != nil {
				return err
			}
			newEtcd = true
			getResp, err = e.client.Get(ctx, e.url.Path, clientv3.WithPrefix())
		}
	}
	if err != nil {
		return fmt.Errorf("bad cluster endpoints, which are not etcd servers: %v", err)
	}
	if len(getResp.Kvs) == 0 {
		kvs, err := config.Marshal(e.url.Path, e.instance)
		if err != nil {
			return fmt.Errorf("path %s marshal error %s", e.url.Path, err)
		}
		for _, kv := range kvs {
			if _, err := e.client.Put(ctx, kv.Key, kv.Value); err != nil {
				return fmt.Errorf("key not found: %s, put error %s", e.url.Path, err)
			}
		}
	} else {
		var kvs []*config.KV
		for _, kv := range getResp.Kvs {
			kvs = append(kvs, &config.KV{
				Key:   string(kv.Key),
				Value: string(kv.Value),
			})
		}
		if newInstance, err := config.Unmarshal(e.url.Path, kvs, e.instance); err != nil {
			return err
		} else {
			e.instance = newInstance
		}
	}
	watch := o.Watch && e.onLoaded != nil
	if newEtcd && watch {
		go e.watch()
	}
	if !watch {
		e.client.Close()
		e.client = nil
	}
	e.onLoaded(e.instance)
	return nil
}

func (e *etcdBackend) watch() {
	wc := e.client.Watch(context.Background(), e.url.Path, clientv3.WithPrefix())
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
			getResp, err := e.client.Get(context.TODO(), e.url.Path, clientv3.WithPrefix())
			if err != nil {
				log.Errorf("Watch channel get prefix err %v", err)
				continue
			}
			var kvs []*config.KV
			for _, kv := range getResp.Kvs {
				kvs = append(kvs, &config.KV{
					Key:   string(kv.Key),
					Value: string(kv.Value),
				})
			}
			if newInstance, err := config.Unmarshal(e.url.Path, kvs, e.instance); err != nil {
				log.Errorf("Watch channel unmarshal err %s", err.Error())
				continue
			} else {
				e.instance = newInstance
				e.onLoaded(newInstance)
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
