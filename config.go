package websvc

type Config struct {
	Host    string `mapstructure:"host"`     // [Optional] Listen host, default is all interfaces
	Port    int    `mapstructure:"port"`     // [Optional] Listen port, default is 80 if SSL is not enabled, 443 otherwise
	SSLCert string `mapstructure:"ssl_cert"` // [Optional] SSL Certificate file path
	SSLKey  string `mapstructure:"ssl_key"`  // [Optional] SSL Key file path
}
