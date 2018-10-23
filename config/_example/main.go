package main

import (
	log "github.com/sirupsen/logrus"
	cfg "github.com/ti/noframe/config"
	_ "github.com/ti/noframe/config/etcd"
	"time"
)

var defaultConfig = Config{
	Addr:     ":9090",
	LogLevel: "debug",
	DataSource: map[string]string{
		"sql":   "mysql://user:pass@127.0.0.1:306/db",
		"cache": "redis://127.0.0.1:6379:127.0.0.1:6380",
	},
	Services: []Service{
		Service{
			Name: "xtimer",
			Url:  "http://xtimer:8080/buket/example",
			Hooks: Hooks{
				Url: "http://127.0.0.1:9090/timeup", Key: "http",
			},
		},
		Service{
			Name: "userinfo",
			Url:  "kv://userinfo",
		},
	},
}

func main() {
	config := &defaultConfig
	err := cfg.Init(cfg.URL("etcd://10.10.134.30:2379,10.10.134.31:2379,10.10.134.32:2379/com/test/demo/"), cfg.WithDefault(config))
	if err != nil {
		panic(err)
	}
	cfg.SetFieldListener("Services[0].Hooks", func(old, new interface{}) {
		log.Println("change from", old.(Hooks), "to", new.(Hooks))
	})
	cfg.SetFieldListener("DataSource.cache", func(old, new interface{}) {
		log.Println("todo something by", new.(string))
	})

	log.Println("change complete", config.Addr)

	time.Sleep(time.Hour)
}

type Config struct {
	Addr       string
	LogLevel   string
	DataSource map[string]string
	Services   []Service
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
