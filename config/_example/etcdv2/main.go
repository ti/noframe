package main

import (
	cfg "github.com/ti/noframe/config"
	_ "github.com/ti/noframe/config/etcdv2"
	"log"
	"time"
)

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
	//只需要填写etcdv2地址即可
	err := cfg.Init(cfg.URL("etcdv2://10.10.98.5:2379/xl/test/config"), cfg.WithDefault(config))
	if err != nil {
		panic(err)
	}
	//监听配置文件变化，你可以安全地使用old.(string)， 库内已经做了封装
	//你也可以将SetFieldListener设置在Init之前，它将通知配置文件读取之后的值
	cfg.SetFieldListener("Services[0].Hooks", func(old, new interface{}) {
		log.Println("配置从", old.(Hooks), "变为", new.(Hooks))
	})
	cfg.SetFieldListener("DataSource.cache", func(old, new interface{}) {
		log.Println("todo something by", new.(string))
	})

	log.Println("配置完成", config.Addr)

	time.Sleep(time.Hour)
}
