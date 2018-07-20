package config

var (
	// std is the name of the standard logger in stdlib `log`
	std = New(map[string]interface{}{})
)

//StandardConfig return instance of default config
func StandardConfig() *Config {
	return std
}

//Init init default config
func Init(opts ...Option) error {
	return std.Init(opts...)
}

//AddFieldListener bind some trigger when config is changed
//if field is "", it add listen whole config
func SetFieldListener(field string, onchange OnChange) {
	std.SetFieldListener(field, onchange)
}

//GetConfig set default config for a instance
func GetConfig() interface{} {
	return std.GetConfig()
}

//AddBackend add backend
func AddBackend(scheme string, backend Backend) {
	std.AddBackend(scheme, backend)
}
