package consul

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	consul "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
	"github.com/ti/noframe/config"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

type consulBackend struct {
	url      *url.URL
	client   *consul.Client
	instance interface{}
	onLoaded config.OnLoaded
}


// New new instance
func New() *consulBackend {
	return &consulBackend{}
}

func init() {
	config.AddBackend("consul", &consulBackend{})
}

// LoadConfig gets the JSON from ETCD and unmarshal it to the config object
func (c *consulBackend) LoadConfig(o config.Options) error {
	if o.DefaultConfig == nil {
		//this should not be happen
		panic("default config can not be nil")
	}
	var err error
	if c.url == nil {
		u, err := url.Parse(o.URL)
		if err != nil {
			return err
		}
		c.url = u
		c.instance = o.DefaultConfig
		c.onLoaded = o.OnLoaded
	}
	var kv *consul.KVPair
	if c.client == nil {
		c.client, err = newClient(c.url)
		if err != nil {
			return err
		}
		kv, _, err = c.client.KV().Get(c.url.Path, nil)
	} else {
		kv, _, err = c.client.KV().Get(c.url.Path, nil)
		if err != nil {
			log.Warnf("etcd get key error ", err, " try 1 time")
			kv, _, err = c.client.KV().Get(c.url.Path, nil)
		}
	}
	if err != nil {
		return fmt.Errorf("bad cluster endpoints, which are not consul servers: %v", err)
	}
	if kv == nil {
		cnfJson, _ := json.MarshalIndent(c.instance, "", "\t")
		kv = &consul.KVPair{
			Key:   c.url.Path,
			Value: cnfJson,
		}
		if _, err := c.client.KV().Put(kv, nil); err != nil {
			return fmt.Errorf("key not found: %s, put error %s", c.url.Path, err)
		}
	} else {
		if err := json.Unmarshal([]byte(kv.Value), c.instance); err != nil {
			return err
		}
	}
	c.onLoaded(c.instance)
	return nil
}

func newClient(uri *url.URL) (*consul.Client, error) {
	cfg := consul.DefaultConfig()
	cfg.Address = uri.Host
	uriQuery := uri.Query()
	cert := uriQuery.Get("cert")
	scheme := uriQuery.Get("scheme")
	if scheme == "https" {
		cfg.Scheme = scheme
	}
	if cert != "" {
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		certs, err := ioutil.ReadFile(cert)
		if err != nil {
			return nil, err
		}
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			log.Warning("No certs appended, using system certs only")
		}

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            rootCAs,
		}

		trans := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		}
		cfg.Scheme = "https"
		cfg.HttpClient.Transport = trans
	}
	datacenter := uriQuery.Get("datacenter")
	if datacenter != "" {
		cfg.Datacenter = datacenter
	}
	if uri.User != nil && uri.User.Username() != "" {
		cfg.HttpAuth = &consul.HttpBasicAuth{
			Username: uri.User.Username(),
		}
		cfg.HttpAuth.Password, _ = uri.User.Password()
	}
	client, err := consul.NewClient(cfg)
	if err != nil {
		return client, err
	}
	return client, nil
}
