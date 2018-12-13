package config

import (
	"reflect"
	"sort"
	"testing"
)

func TestGetKeysKind(t *testing.T) {
	keysKind, err := GetKeysKind("/dir/test", &testKV)
	if err != nil {
		t.Fail()
	}
	expect := map[string]fieldKind{
		"/dir/test":              {"", reflect.Struct},
		"/dir/test/data_source/": {"DataSource", reflect.Map},
		"/services/test/a":       {"Services", reflect.Slice},
	}
	if !reflect.DeepEqual(keysKind, expect) {
		t.Fail()
	}
}

func TestMarshal(t *testing.T) {
	data, err := Marshal("/dir/test", &testKV)
	if err != nil {
		t.Fail()
	}
	expect := []*KV{
		{
			Key:   "/dir/test",
			Value: `{"Addr":":9090","LogLevel":"debug","test_config":null}`,
		},
		{
			Key:   "/dir/test/data_source/cache",
			Value: `"redis://127.0.0.1:6379:127.0.0.1:6380"`,
		},
		{
			Key:   "/dir/test/data_source/sql",
			Value: `"mysql://user:pass@127.0.0.1:306/db"`,
		},
		{
			Key:   "/services/test/a",
			Value: `[{"Name":"serviceA","Url":"http://servicea:8080/buket/example","Hooks":{"Url":"http://127.0.0.1:9090/timeup","Key":"http"}},{"Name":"userinfo","Url":"kv://userinfo","Hooks":{"Url":"","Key":""}}]`,
		},
	}
	dataSort := kvSort(data)
	sort.Sort(dataSort)
	for i, kv := range dataSort {
		if kv.Key != expect[i].Key {
			t.Fatalf("key %s does not match expect %s", kv.Key, expect[i].Key)
		}
		if kv.Value != expect[i].Value {
			t.Fatalf("value %s does not match expect %s", kv.Value, expect[i].Value)
		}
	}
	data, err = Marshal("/dir/test/", &testKV)
	if err != nil {
		t.Fail()
	}

	expect = []*KV{
		{
			Key:   "/dir/test/data_source/cache",
			Value: `"redis://127.0.0.1:6379:127.0.0.1:6380"`,
		},
		{
			Key:   "/dir/test/data_source/sql",
			Value: `"mysql://user:pass@127.0.0.1:306/db"`,
		},
		{
			Key:   "/services/test/a",
			Value: `[{"Name":"serviceA","Url":"http://servicea:8080/buket/example","Hooks":{"Url":"http://127.0.0.1:9090/timeup","Key":"http"}},{"Name":"userinfo","Url":"kv://userinfo","Hooks":{"Url":"","Key":""}}]`,
		},
		{
			Key:   "Addr",
			Value: `":9090"`,
		},
		{
			Key:   "LogLevel",
			Value: `"debug"`,
		},
		{
			Key:   "test_config",
			Value: `null`,
		},
	}
	dataSort = kvSort(data)
	sort.Sort(dataSort)
	for i, kv := range dataSort {
		if kv.Key != expect[i].Key {
			t.Fatalf("key %s does not match expect %s", kv.Key, expect[i].Key)
		}
		if kv.Value != expect[i].Value {
			t.Fatalf("value %s does not match expect %s", kv.Value, expect[i].Value)
		}
	}
}

var testKV = TestKV{
	Addr:     ":9090",
	LogLevel: "debug",
	DataSource: map[string]string{
		"sql":   "mysql://user:pass@127.0.0.1:306/db",
		"cache": "redis://127.0.0.1:6379:127.0.0.1:6380",
	},
	Services: []Service{
		{
			Name: "serviceA",
			Url:  "http://servicea:8080/buket/example",
			Hooks: Hooks{
				Url: "http://127.0.0.1:9090/timeup",
				Key: "http",
			},
		},
		{
			Name: "userinfo",
			Url:  "kv://userinfo",
		},
	},
}

type TestKV struct {
	Addr       string
	LogLevel   string
	DataSource map[string]string `config:"data_source/"`
	Services   []Service         `config:"/services/test/a"`
	TestConfig map[string]string `json:"test_config"`
}

type Service struct {
	Name  string
	Url   string
	Hooks Hooks
}

type Hooks struct {
	Url string
	Key string
}
