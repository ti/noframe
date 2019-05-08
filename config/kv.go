package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
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

const tagName = "config"

type fieldKind struct {
	Field string
	Kind  reflect.Kind
}

func getFiledTag(tagName string, f reflect.StructField) string {
	fKey := f.Name
	if t := f.Tag.Get(tagName); t != "" {
		indexDot := strings.Index(t, ",")
		if indexDot < 0 {
			fKey = t
		} else if indexDot > 0 {
			fKey = t[:indexDot]
		} else {
			fKey = strings.Split(t, ",")[0]
		}
	}
	return fKey
}

//GetPrefixKeys get the prefix keys by target
func GetPrefixKeys(key string, target interface{}) (ret []string) {
	keysKind, err := getKeysKind(key, target)
	if err != nil {
		return
	}
	if strings.HasSuffix(key, "/") {
		ret = []string{key}
		for k := range keysKind {
			if !strings.HasPrefix(k, key) {
				ret = append(ret, k)
			}
		}
	} else {
		for k := range keysKind {
			ret = append(ret, k)
		}
	}
	return
}

//getKeysKind get all the keys and kind for list usage
func getKeysKind(key string, target interface{}) (keyTypeMap map[string]fieldKind, err error) {
	src := getReflectValue(target)
	mainKind := src.Kind()
	keyTypeMap = make(map[string]fieldKind)
	keyTypeMap[key] = fieldKind{
		Field: "",
		Kind:  mainKind,
	}
	if mainKind == reflect.Struct {
		rootKey := key
		if !strings.HasSuffix(rootKey, "/") {
			rootKey += "/"
		}
		size := src.NumField()
		for i := 0; i < size; i++ {
			f := src.Type().Field(i)
			fKind := f.Type.Kind()
			if t := f.Tag.Get(tagName); t != "" {
				jsonTag := getFiledTag("json", f)
				kvTag := getFiledTag(tagName, f)
				if strings.HasPrefix(kvTag, "/") {
					keyTypeMap[kvTag] = fieldKind{
						Field: jsonTag,
						Kind:  fKind,
					}
				} else if strings.Contains(kvTag, "/") {
					keyTypeMap[rootKey+kvTag] = fieldKind{
						Field: jsonTag,
						Kind:  fKind,
					}
				}
			}
		}
	}
	return
}

func kvsToJSON(kvs []*KV) string {
	ret := "{"
	kvsLen := len(kvs)
	for i, kv := range kvs {
		ret += `"` + kv.Key + `":` + kv.Value
		if i < kvsLen-1 {
			ret += ","
		}
	}
	ret += "}"
	return ret
}

func getReflectValue(i interface{}) (v reflect.Value) {
	if reflect.ValueOf(i).Kind() != reflect.Ptr {
		v = reflect.ValueOf(i)
	} else {
		v = reflect.ValueOf(reflect.Indirect(reflect.ValueOf(i)).Interface())
	}
	return
}

func mapKVMarshal(key string, src reflect.Value) (kvs []*KV, err error) {
	keys := src.MapKeys()
	for _, k := range keys {
		value := src.MapIndex(k)
		b, err := json.Marshal(value.Interface())
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, &KV{
			Key:   key + k.String(),
			Value: string(b),
		})
	}
	return
}

func sliceKVMarshal(key string, src reflect.Value) (kvs []*KV, err error) {
	size := src.Len()
	for i := 0; i < size; i++ {
		value := src.Index(i)
		b, err := json.Marshal(value.Interface())
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, &KV{
			Key:   key + strconv.Itoa(i),
			Value: string(b),
		})
	}
	return
}

func simpleKVMarshal(key string, target interface{}) ([]*KV, error) {
	b, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	return []*KV{
		{
			Key:   key,
			Value: string(b),
		},
	}, nil
}

//Marshal unmarshal interface to kv array
func Marshal(key string, target interface{}) (kvs []*KV, err error) {
	src := getReflectValue(target)
	mainKind := src.Kind()
	ifMainIsDir := strings.HasSuffix(key, "/")
	if mainKind == reflect.Struct {
		rootKey := key
		if !strings.HasSuffix(rootKey, "/") {
			rootKey += "/"
		}
		size := src.NumField()
		var mainKvs []*KV
		for i := 0; i < size; i++ {
			f := src.Type().Field(i)
			fKind := f.Type.Kind()
			jsonFiledKey := getFiledTag("json", f)
			fKey := getFiledTag(tagName, f)
			data := src.Field(i).Interface()
			var childKvs []*KV
			var mainChildKvs []*KV
			if strings.Contains(fKey, "/") {
				if !strings.HasPrefix(fKey, "/") {
					fKey = rootKey + fKey
				}
				childKvs, err = uniMarshal(fKey, fKind, data)
			} else {
				if ifMainIsDir {
					childKvs, err = simpleKVMarshal(jsonFiledKey, data)
				} else {
					mainChildKvs, err = simpleKVMarshal(jsonFiledKey, data)
				}
			}
			if err != nil {
				return nil, err
			}
			for _, kv := range childKvs {
				kvs = append(kvs, kv)
			}
			for _, kv := range mainChildKvs {
				mainKvs = append(mainKvs, kv)
			}
		}
		if len(mainKvs) > 0 {
			kvs = append([]*KV{
				{
					Key:   key,
					Value: kvsToJSON(mainKvs),
				},
			}, kvs...)
		}
	} else {
		if !ifMainIsDir {
			return simpleKVMarshal(key, target)
		}
		switch mainKind {
		case reflect.Map:
			kvs, err = mapKVMarshal(key, src)
		case reflect.Slice:
			kvs, err = sliceKVMarshal(key, src)
		default:
			return simpleKVMarshal(key, target)
		}
	}
	return
}

func uniMarshal(fKey string, fKind reflect.Kind, data interface{}) (childKvs []*KV, err error) {
	if !strings.HasSuffix(fKey, "/") {
		childKvs, err = simpleKVMarshal(fKey, data)
	} else {
		switch fKind {
		case reflect.Map:
			childKvs, err = mapKVMarshal(fKey, getReflectValue(data))
		case reflect.Slice:
			childKvs, err = sliceKVMarshal(fKey, getReflectValue(data))
		default:
			childKvs, err = simpleKVMarshal(fKey, data)
		}
	}
	return
}
func mapKVUnmarshal(key string, kvs []*KV, distValue interface{}) (err error) {
	ret := "{"
	maxIndex := len(kvs) - 1
	for i, kv := range kvs {
		fKey := kv.Key
		if strings.HasPrefix(fKey, key) {
			fKey = fKey[len(key):]
		}
		if fKey != "" && !strings.Contains(fKey, "/") {
			part := `"` + fKey + `":` + kv.Value
			if i == maxIndex {
				part += "}"
			} else {
				part += ","
			}
			ret += part
		}
	}
	return json.Unmarshal([]byte(ret), distValue)
}

func sliceKVUnmarshal(key string, kvs []*KV, distValue interface{}) (err error) {
	//sort it first
	sort.Sort(kvSort(kvs))
	ret := "["
	maxIndex := len(kvs) - 1
	for i, kv := range kvs {
		fKey := kv.Key
		if strings.HasPrefix(fKey, key) {
			fKey = fKey[len(key):]
		}
		if fKey != "" && !strings.Contains(fKey, "/") {
			ret += kv.Value
			if i == maxIndex {
				ret += "]"
			} else {
				ret += ","
			}
		}
	}
	return json.Unmarshal([]byte(ret), distValue)
}

//Unmarshal unmarshal interface to kv array
func Unmarshal(key string, kvs []*KV, target interface{}) (err error) {
	if len(kvs) == 0 {
		return errors.New("kvs size is 0")
	}
	src := getReflectValue(target)
	mainKind := src.Kind()
	ifMainIsDir := strings.HasSuffix(key, "/")
	targetType := reflect.Indirect(reflect.ValueOf(target)).Type()
	distValue := reflect.New(targetType).Interface()
	if reflect.ValueOf(target).Kind() != reflect.Ptr {
		distValue = reflect.Indirect(reflect.ValueOf(target)).Interface()
	}
	if mainKind == reflect.Struct {
		var js string
		js, err = unmarshalKVStructToJson(key, target, kvs)
		if err == nil {
			err = json.Unmarshal([]byte(js), distValue)
		}
	} else {
		if !ifMainIsDir {
			err = json.Unmarshal([]byte(kvs[0].Value), distValue)
		} else {
			switch mainKind {
			case reflect.Map:
				err = mapKVUnmarshal(key, kvs, distValue)
			case reflect.Slice:
				err = sliceKVUnmarshal(key, kvs, distValue)
			default:
				err = json.Unmarshal([]byte(kvs[0].Value), distValue)
			}
		}
	}
	if err == nil {
		cloneValue(distValue, target)
	}
	return
}

func unmarshalKVStructToJson(key string, target interface{}, kvs []*KV) (js string, err error) {
	rootKey := key
	if !strings.HasSuffix(rootKey, "/") {
		rootKey += "/"
	}
	var keysKind map[string]fieldKind
	keysKind, err = getKeysKind(key, target)
	if err != nil {
		return
	}
	type KindKVs struct {
		KVS   []*KV
		Kind  reflect.Kind
		Field string
	}
	var childKvs = make(map[string]*KindKVs)
	var parts []string
	for _, kv := range kvs {
		fKey := kv.Key
		fValue := kv.Value
		if len(fValue) == 0 {
			continue
		}
		firstChar := fValue[0]
		if !(firstChar == '{' || firstChar == '[') {
			fValue = fmt.Sprintf(`"%s"`, fValue)
		}
		if fKey == key {
			parts = append(parts, fValue[1:len(fValue)-1])
		} else if !strings.Contains(fKey, "/") {
			parts = append(parts, `"`+fKey+`":`+fValue)
		} else if strings.HasPrefix(fKey, key) {
			fKey = fKey[len(rootKey):]
			index := strings.LastIndex(fKey, "/")
			if index > 0 {
				childKey := fKey[0 : index+1]
				childKind, ok := keysKind[rootKey+childKey]
				if ok {
					childKV, ok := childKvs[childKey]
					if ok {
						childKV.KVS = append(childKV.KVS, kv)
					} else {
						childKvs[childKey] = &KindKVs{
							Kind:  childKind.Kind,
							Field: childKind.Field,
							KVS:   []*KV{kv},
						}
					}
				} else {
					filedFKey := rootKey + fKey
					childKind, ok := keysKind[filedFKey]
					if ok {
						parts = append(parts, `"`+childKind.Field+`":`+fValue)
					} else {
						parts = append(parts, `"`+fKey+`":`+fValue)
					}
				}
			} else {
				parts = append(parts, `"`+fKey+`":`+fValue)
			}
		} else {
			kindKey, ok := keysKind[fKey]
			if ok {
				parts = append(parts, `"`+kindKey.Field+`":`+fValue)
			} else {
				index := strings.LastIndex(fKey, "/")
				childKey := fKey[0 : index+1]
				childKind, ok := keysKind[childKey]
				if ok {
					childKV, ok := childKvs[childKey]
					if ok {
						childKV.KVS = append(childKV.KVS, kv)
					} else {
						childKvs[childKey] = &KindKVs{
							Kind:  childKind.Kind,
							Field: childKind.Field,
							KVS:   []*KV{kv},
						}
					}
				}
			}
		}
	}
	for _, v := range childKvs {
		js := partsToJSON(v.Kind, v.KVS)
		parts = append(parts, `"`+v.Field+`":`+js)
	}
	js = "{" + strings.Join(parts, ",") + "}"
	return
}

func partsToJSON(kind reflect.Kind, kvs []*KV) string {
	switch kind {
	case reflect.Map, reflect.Struct:
		ret := "{"
		maxIndex := len(kvs) - 1
		for i, kv := range kvs {
			index := strings.LastIndex(kv.Key, "/")
			mapKey := kv.Key[index+1:]
			part := `"` + mapKey + `":` + kv.Value
			if i == maxIndex {
				part += "}"
			} else {
				part += ","
			}
			ret += part
		}
		return ret
	case reflect.Slice:
		//sort it first
		sort.Sort(kvSort(kvs))
		ret := "["
		maxIndex := len(kvs) - 1
		for i, kv := range kvs {
			ret += kv.Value
			if i == maxIndex {
				ret += "]"
			} else {
				ret += ","
			}
		}
		return ret
	default:
		return "{}"
	}
}

func cloneValue(src interface{}, dest interface{}) {
	x := reflect.ValueOf(src)
	if x.Kind() == reflect.Ptr {
		starX := x.Elem()
		y := reflect.New(starX.Type())
		starY := y.Elem()
		starY.Set(starX)
		reflect.ValueOf(dest).Elem().Set(y.Elem())
	}
}
