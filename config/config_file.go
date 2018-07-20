package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
)

const fileScheme = "file"

type fileBackend struct {
	path string
}

// LoadConfig get config from file
func (f *fileBackend) LoadConfig(o Options) error {
	if o.DefaultConfig == nil {
		//this should not be happen
		panic("default config can not be nil")
	}
	if f.path == "" {
		u, err := url.Parse(o.URL)
		if err != nil {
			return err
		}
		f.path = u.Host + u.Path
	}
	config := o.DefaultConfig
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
		if writeErr := ioutil.WriteFile(f.path, marshal(config, ext), os.FileMode(0700)); writeErr != nil {
			return fmt.Errorf("try to open file %s, try to write default config config file rror %s", err, writeErr)
		}
		o.OnLoaded(config)
		return nil
	}

	err = unmarshal(bytes, config, ext)
	if err != nil {
		return err
	}
	o.OnLoaded(config)
	return nil
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
