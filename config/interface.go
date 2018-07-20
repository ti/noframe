package config

// Backend defines a configuration backend, implement this interface
// to support additional backends, such as etcd, consul, file ...
type Backend interface {
	LoadConfig(options Options) error
}

//OnChange Trigger On config change function
type OnChange func(pre, current interface{})

//OnLoaded Trigger On config is loaded
type OnLoaded func(cfg interface{})
