package config

import (
	"errors"
	"strings"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
)

//KV the key value of a interface
type KV struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type kvSort []*KV

func (s kvSort) Len() int           { return len(s) }
func (s kvSort) Less(i, j int) bool { return s[i].Key < s[j].Key }
func (s kvSort) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

//Unmarshal unmarshal interface to kv array
func Unmarshal(key string, kvs []*KV, target interface{}) error {
	if len(kvs) == 0 {
		return errors.New("kvs size is 0")
	}
	if !strings.HasSuffix(key, "/") {
		return json.Unmarshal([]byte(kvs[0].Value), target)
	}
	src := reflect.ValueOf(target)
	if src.Kind() == reflect.Ptr {
		src = reflect.Indirect(src)
	}
	lenKey := len(key)
	switch k := src.Kind(); k {
	case reflect.Map, reflect.Struct:
		ret := "{"
		maxIndex := len(kvs) - 1
		for i, kv := range kvs {
			mapKey := kv.Key[lenKey:]
			if strings.HasPrefix(kv.Key, key) && !strings.Contains(mapKey, "/") {
				part := `"` + mapKey + `":` + kv.Value
				if i == maxIndex {
					part += "}"
				} else {
					part += ","
				}
				ret += part
			}
		}
		return json.Unmarshal([]byte(ret), target)
	case reflect.Slice:
		//sort it first
		sort.Sort(kvSort(kvs))
		ret := "["
		maxIndex := len(kvs) - 1
		for i, kv := range kvs {
			if strings.HasPrefix(kv.Key, key) && !strings.Contains(kv.Key[lenKey:], "/") {
				ret += kv.Value
				if i == maxIndex {
					ret += "]"
				} else {
					ret += ","
				}
			}
		}
		return json.Unmarshal([]byte(ret), target)
	default:
		return json.Unmarshal([]byte(kvs[0].Value), target)
	}

	return nil

}

//Marshal unmarshal interface to kv array
func Marshal(key string, target interface{}) ([]*KV, error) {
	var simpleKv = func() ([]*KV, error) {
		b, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		return []*KV{
			&KV{
				Key:   key,
				Value: string(b),
			},
		}, nil
	}
	if !strings.HasSuffix(key, "/") {
		return simpleKv()
	}
	src := reflect.ValueOf(target)
	if src.Kind() == reflect.Ptr {
		src = reflect.Indirect(src)
	}
	var ret []*KV
	switch k := src.Kind(); k {
	case reflect.Map:
		keys := src.MapKeys()
		for _, k := range keys {
			value := src.MapIndex(k)
			b, err := json.Marshal(value.Interface())
			if err != nil {
				return nil, err
			}
			ret = append(ret, &KV{
				Key:   key + k.String(),
				Value: string(b),
			})
		}
	case reflect.Slice:
		size := src.Len()
		for i := 0; i < size; i++ {
			value := src.Index(i)
			b, err := json.Marshal(value.Interface())
			if err != nil {
				return nil, err
			}
			ret = append(ret, &KV{
				Key:   key + strconv.Itoa(i),
				Value: string(b),
			})
		}
	case reflect.Struct:
		size := src.NumField()
		for i := 0; i < size; i++ {
			f := src.Field(i)
			b, err := json.Marshal(f.Interface())
			if err != nil {
				return nil, err
			}
			fType := src.Type().Field(i)
			fKey := fType.Name
			if t := fType.Tag.Get("json"); t != "" {
				indexDot := strings.Index(t, ",")
				if indexDot < 0 {
					fKey = t
				} else if indexDot > 0 {
					fKey = t[:indexDot]
				}
				fKey = strings.Split(t, ",")[0]
			}
			ret = append(ret, &KV{
				Key:   key + fKey,
				Value: string(b),
			})
		}
	default:
		return simpleKv()
	}

	return ret, nil
}
