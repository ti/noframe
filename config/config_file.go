package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
)

// FileScheme the sheme for file
const fileScheme = "file"

type fileBackend struct {
	path     string
	instance interface{}
	onLoaded OnLoaded
	loaded   bool
}

func init() {
	AddBackend(fileScheme, &fileBackend{})
}

func (f *fileBackend) reloadFile() error {
	bytes, err := ioutil.ReadFile(f.path)
	if err != nil {
		return err
	}
	ext := filepath.Ext(f.path)
	err = unmarshal(bytes, f.instance, ext)
	if err != nil {
		return err
	}
	return nil
}

// LoadConfig get config from file
func (f *fileBackend) LoadConfig(o Options) error {
	if o.DefaultConfig == nil {
		//this should not be happen
		panic("default config can not be nil")
	}
	u, err := url.Parse(o.URL)
	if err != nil {
		return err
	}
	if f.path == "" {
		f.path = u.Host + u.Path
	}
	f.instance = o.DefaultConfig
	f.onLoaded = o.OnLoaded

	cfg := o.DefaultConfig
	ext := filepath.Ext(f.path)
	bytes, err := ioutil.ReadFile(f.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		fileDir := filepath.Dir(f.path)
		if _, pathStatErr := os.Stat(fileDir); pathStatErr != nil {
			if !os.IsNotExist(pathStatErr) {
				return fmt.Errorf("try to open file error %s, try to stat dir %s,  error %s", err, fileDir, pathStatErr)
			}
			if mkdirError := os.MkdirAll(fileDir, os.FileMode(0700)); mkdirError != nil {
				return fmt.Errorf("try to open file %s, try to mkdir %s,  error %s", err, f.path, mkdirError)
			}
		}
		if writeErr := ioutil.WriteFile(f.path, marshal(cfg, ext), os.FileMode(0700)); writeErr != nil {
			return fmt.Errorf("try to open file %s, try to write default config config file rror %s", err, writeErr)
		}
		o.OnLoaded(cfg)
		return nil
	}
	err = unmarshal(bytes, cfg, ext)
	if err != nil {
		return err
	}
	prefixKeys := GetPrefixKeys(f.path, o.DefaultConfig)
	if o.Watch && !f.loaded {
		go f.watch(context.Background(), f.path, prefixKeys)
	}
	f.loaded = true
	o.OnLoaded(cfg)
	return nil
}

func (f *fileBackend) watch(ctx context.Context, rootKey string, keys []string) {
	watcher, err := fsnotify.NewWatcher()
	l := log.WithField("action", "watch_file").WithField("root", rootKey)
	if err != nil {
		l.Error("watch file %s error ", rootKey, err)
	}
	defer watcher.Close()
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Chmod {
					return
				}
				err = f.reloadFile()
				if err != nil {
					l.Error(err)
					return
				}
				f.onLoaded(f.instance)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				l.Error(err)
			}
		}
	}()
	err = watcher.Add(rootKey)
	if err != nil {
		panic(err)
	}
	<-done
}

func marshal(v interface{}, ext string) (ret []byte) {
	if ext == ".json" {
		ret, _ = json.MarshalIndent(v, "", "\t")
		return
	}
	ret, _ = yaml.Marshal(v)
	return
}

func unmarshal(in []byte, out interface{}, ext string) error {
	if ext == ".json" {
		return json.Unmarshal(in, out)
	}
	return yaml.Unmarshal(in, out)
}
