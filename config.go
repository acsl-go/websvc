package websvc

type Config struct {
	Host    string `mapstructure:"host" json:"host" yaml:"host"`             // [Optional] Listen host, default is all interfaces
	Port    int    `mapstructure:"port" json:"port" yaml:"port"`             // [Optional] Listen port, default is 80 if SSL is not enabled, 443 otherwise
	SSLCert string `mapstructure:"ssl_cert" json:"ssl_cert" yaml:"ssl_cert"` // [Optional] SSL Certificate file path
	SSLKey  string `mapstructure:"ssl_key" json:"ssl_key" yaml:"ssl_key"`    // [Optional] SSL Key file path
}

func (c *Config) IsSSL() bool {
	return c.SSLCert != "" && c.SSLKey != ""
}
