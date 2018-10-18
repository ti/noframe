package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	etcd "github.com/etcd-io/etcd/client"
	"github.com/etcd-io/etcd/pkg/transport"
	log "github.com/sirupsen/logrus"
	"github.com/ti/noframe/config"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

type etcdBackend struct {
	url      *url.URL
	client   etcd.Client
	keyApis  etcd.KeysAPI
	instance interface{}
	onLoaded config.OnLoaded
}

func init() {
	config.AddBackend("etcdv2", &etcdBackend{})
}

// LoadConfig gets the JSON from ETCD and unmarshals it to the config object
func (e *etcdBackend) LoadConfig(o config.Options) error {
	if o.DefaultConfig == nil {
		//this should not be happen
		panic("default config can not be nil")
	}
	var err error
	var getResp *etcd.Response
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

	if e.keyApis == nil {
		//first time to load config
		e.client, err = newEtcdClient(e.url)
		if err != nil {
			return err
		}
		e.keyApis = etcd.NewKeysAPI(e.client)
		getResp, err = e.keyApis.Get(context.Background(), e.url.Path, nil)
		newEtcd = true
	} else {
		getResp, err = e.keyApis.Get(context.Background(), e.url.Path, nil)
		if err != nil && !etcd.IsKeyNotFound(err) {
			e.client = nil
			log.Warnf("etcd v2 get key error ", err, " try 1 time")
			e.client, err = newEtcdClient(e.url)
			if err != nil {
				return err
			}
			e.keyApis = etcd.NewKeysAPI(e.client)
			getResp, err = e.keyApis.Get(context.Background(), e.url.Path, nil)
			newEtcd = true
		}
	}
	if err != nil && !etcd.IsKeyNotFound(err) {
		return fmt.Errorf("bad cluster endpoints, which are not etcd servers: %v", err)
	}
	if etcd.IsKeyNotFound(err) {
		cnfJson, _ := json.MarshalIndent(e.instance, "", "\t")
		if _, err := e.keyApis.Set(ctx, e.url.Path, string(cnfJson), nil); err != nil {
			return fmt.Errorf("key not found: %s, put error %s", e.url.Path, err)
		}
	} else {
		if err := json.Unmarshal([]byte(getResp.Node.Value), e.instance); err != nil {
			return err
		}
	}
	watch := o.Watch && e.onLoaded != nil
	if newEtcd && watch {
		go e.watch()
	}
	e.onLoaded(e.instance)
	return nil
}

func (e *etcdBackend) watch() {
	ctx := context.TODO()
	wc := e.keyApis.Watcher(e.url.Path, &etcd.WatcherOptions{AfterIndex: 0, Recursive: true})
	for {
		rsp, err := wc.Next(ctx)
		if err != nil && ctx.Err() != nil {
			log.Error("etcd v2 watch error ", err)
			return
		}
		if rsp.Node.Dir {
			continue
		}
		switch rsp.Action {
		case "set", "update":
			if err := json.Unmarshal([]byte(rsp.Node.Value), e.instance); err == nil {
				e.onLoaded(e.instance)
			}
		}
	}
}

func newEtcdClient(etcdUri *url.URL) (etcd.Client, error) {
	hosts := strings.Split(etcdUri.Host, ",")
	for i, v := range hosts {
		hosts[i] = "http://" + v
	}
	etcdConfig := etcd.Config{
		Endpoints: hosts,
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

		t := &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     tlsConfig,
		}
		runtime.SetFinalizer(&t, func(tr **http.Transport) {
			(*tr).CloseIdleConnections()
		})
		etcdConfig.Transport = t
	}
	return etcd.New(etcdConfig)
}
