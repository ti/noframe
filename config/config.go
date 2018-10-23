package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

//Config the struct of config instance
type Config struct {
	triggers map[string]OnChange
	backEnds map[string]Backend
	//some backend of config, you can use file, etcd2, etcd3, consul ...
	instance interface{}
	//pre instance of config
	preInstance interface{}
	onChange    OnChange
	mu          sync.Mutex
}

//Init init config by url
func (c *Config) Init(opts ...Option) error {
	var options Options
	for _, o := range opts {
		o(&options)
	}
	if options.DefaultConfig == nil {
		options.DefaultConfig = c.instance
	} else {
		c.instance = options.DefaultConfig
	}
	if options.URL == "" {
		if c.instance != nil {
			c.onReloaded(c.instance)
			return nil
		}
		return errors.New("config not set")
	}
	var backend Backend
	if options.scheme == "" {
		backend = &fileBackend{path: options.URL}
	} else {
		var ok bool
		backend, ok = c.backEnds[options.scheme]
		if !ok {
			return fmt.Errorf("[%s] is not a valid backend url", options.URL)
		}
	}
	options.OnLoaded = c.onReloaded
	if err := backend.LoadConfig(options); err != nil {
		return err
	}
	if options.ReloadDelay > time.Second {
		go func() {
			for {

				// Delay after each request
				<-time.After(options.ReloadDelay)
				// Attempt to reload the config
				err := backend.LoadConfig(options)
				if err != nil {
					log.Error(err)
					continue
				}
			}
		}()
	}

	return nil
}

//New new config use default config for config
func New(defaultConfig interface{}) *Config {
	return &Config{
		instance: defaultConfig,
		triggers: make(map[string]OnChange),
		backEnds: map[string]Backend{fileScheme: &fileBackend{}},
	}
}

//GetConfig set default config for a instance
func (c *Config) GetConfig() interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.instance
}

//AddBackend add backend
func (c *Config) AddBackend(scheme string, backend Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backEnds[scheme] = backend
}

//SetFieldListener bind some trigger when config is changed
func (c *Config) SetFieldListener(field string, onChange OnChange) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if onChange == nil {
		//remove listener
		delete(c.triggers, field)
		return
	}
	c.triggers[field] = onChange
}

//onReloaded notify some trigger on data
func (c *Config) onReloaded(cfg interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hasPreInstance := c.preInstance != nil
	newConfig := reflect.Indirect(reflect.ValueOf(cfg)).Interface()
	if hasPreInstance && reflect.DeepEqual(c.preInstance, newConfig) {
		return
	}
	for field, onChange := range c.triggers {
		var oldValue interface{}
		if hasPreInstance {
			oldValue, _ = getFieldValue(c.preInstance, field)
		}
		newValue, err := getFieldValue(newConfig, field)
		if err != nil {
			log.Warnf("can not get config by field %s", err)
			continue
		}
		if newValue == nil {
			if oldValue != nil {
				tx := reflect.Indirect(reflect.ValueOf(oldValue)).Type()
				newValue = reflect.New(tx).Interface()
				if reflect.ValueOf(oldValue).Kind() != reflect.Ptr {
					newValue = reflect.Indirect(reflect.ValueOf(newValue)).Interface()
				}
				onChange(oldValue, newValue)
			}
		} else {
			if !hasPreInstance || !reflect.DeepEqual(oldValue, newValue) {
				if oldValue == nil {
					tx := reflect.Indirect(reflect.ValueOf(newValue)).Type()
					oldValue = reflect.New(tx).Interface()
					if reflect.ValueOf(newValue).Kind() != reflect.Ptr {
						oldValue = reflect.Indirect(reflect.ValueOf(oldValue)).Interface()
					}
				}
				onChange(oldValue, newValue)
			}
		}
	}
	c.preInstance = clone(cfg)
}

func getFieldValue(src interface{}, path string) (interface{}, error) {
	dist, err := getFieldValueReflect(reflect.ValueOf(src), compile(path))
	if err != nil {
		if err == errorOutOfRange {
			return nil, nil
		}
		return nil, fmt.Errorf("path %s errr for %s", path, err)
	}
	return dist.Interface(), nil
}

var errorOutOfRange = errors.New("out of range")

func getFieldValueReflect(src reflect.Value, paths []string) (reflect.Value, error) {
	if len(paths) == 0 {
		return src, nil
	}
	if !src.IsValid() {
		return reflect.Value{}, fmt.Errorf("%s is not valid", paths)
	}
	key := paths[0]
	switch k := src.Kind(); k {
	case reflect.Map:
		paths = paths[1:]
		src = src.MapIndex(reflect.ValueOf(key))
	case reflect.Struct:
		paths = paths[1:]
		src = src.FieldByName(key)
	case reflect.Slice:
		paths = paths[1:]
		n, en := strconv.Atoi(key)
		if en != nil {
			return reflect.Value{}, fmt.Errorf("%s is not a number %s", key, en)
		}
		if src.Len() < n+1 {
			return reflect.Value{}, errorOutOfRange
		}
		src = src.Index(n)
	case reflect.Ptr:
		src = reflect.Indirect(src)
	default:
		return reflect.Value{}, fmt.Errorf("%s is not supported", k)
	}
	return getFieldValueReflect(src, paths)
}

func compile(src string) []string {
	var a []string
	var p int
	l := len(src)
	if l == 0 {
		return a
	}
	var v uint8
	for i := 0; i < l; i++ {
		v = src[i]
		if v == ']' {
			a = append(a, src[p:i])
			p = i + 2
			i++
			continue
		}
		if v == '.' || v == '[' {
			a = append(a, src[p:i])
			p = i + 1
		}
	}
	if p < l {
		a = append(a, src[p:])
	}
	return a
}

// copy fully copy config instance, include map
func clone(src interface{}) interface{} {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	dec := json.NewDecoder(&buf)
	err := enc.Encode(src)
	if err != nil {
		return nil
	}
	t := reflect.Indirect(reflect.ValueOf(src)).Type()
	dist := reflect.New(t).Interface()
	err = dec.Decode(&dist)
	if err != nil {
		return nil
	}
	return reflect.Indirect(reflect.ValueOf(dist)).Interface()
}
