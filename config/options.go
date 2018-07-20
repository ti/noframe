package config

import (
	"context"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

//Options the Options of config
type Options struct {
	URL     string
	Timeout time.Duration
	//ReloadDelay refresh the config after some time, it used for etcd, consul for the backend may not notify the config
	ReloadDelay time.Duration
	//Watch is watch is true, the etcd will keep connection for mq notify
	Watch bool
	//DefaultConfig default config
	DefaultConfig interface{}
	//OnLoaded ! do not set this Manually, this is internal usage
	OnLoaded OnLoaded
	// Other options for implementations of the interface
	// can be stored in a context
	Context context.Context

	scheme string
}

//Option is just Option functions
type Option func(*Options)

// URL is the registry addresses to use
// exp: etcd://127.0.0.1:6379,127.0.0.1:7379/xl/config/key
// exp: file://conf/service.yaml
func URL(uri string) Option {
	return func(o *Options) {
		u, err := url.Parse(uri)
		if err != nil {
			panic(err)
		}
		o.URL = uri
		o.scheme = u.Scheme
		if o.scheme == fileScheme || o.scheme == "" {
			return
		}
		//default config
		o.Timeout = 30 * time.Second
		o.ReloadDelay = time.Hour
		o.Watch = true
		//load config by url
		if ttl, _ := strconv.Atoi(u.Query().Get("ttl")); ttl != 0 {
			if ttl < 0 {
				o.ReloadDelay = 0
			} else {
				o.ReloadDelay = time.Second * time.Duration(ttl)
				if ttl <= 60 {
					o.Watch = false
				}
				log.Println("watch?", o.Watch)
			}
		}
		if w := u.Query().Get("watch"); w != "" {
			o.Watch = !(w == "false")
		}
		if t := u.Query().Get("timeout"); t != "" {
			timeout, _ := strconv.Atoi(t)
			o.Timeout = time.Second * time.Duration(timeout)
		}
		o.scheme = u.Scheme
	}
}

//WithDefault set default config of the instance
func WithDefault(defaultConfig interface{}) Option {
	return func(o *Options) {
		if reflect.ValueOf(defaultConfig).Kind() != reflect.Ptr {
			panic("default config should be a pointer")
		}
		o.DefaultConfig = defaultConfig
	}
}

//Timeout load timeout
func Timeout(t time.Duration) Option {
	return func(o *Options) {
		o.Timeout = t
	}
}

//Watch if keep the etcd connections and watch notify
func Watch(w bool) Option {
	return func(o *Options) {
		o.Watch = w
	}
}

//Timeout load timeout
func ReloadDelay(t time.Duration) Option {
	return func(o *Options) {
		o.ReloadDelay = t
	}
}
