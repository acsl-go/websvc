package websvc

type Config struct {
	Host           string   `mapstructure:"host" json:"host" yaml:"host"`                                     // [Optional] Listen host, default is all interfaces
	Port           int      `mapstructure:"port" json:"port" yaml:"port"`                                     // [Optional] Listen port, default is 80 if SSL is not enabled, 443 otherwise
	SSLCert        string   `mapstructure:"ssl_cert" json:"ssl_cert" yaml:"ssl_cert"`                         // [Optional] SSL Certificate content or file path
	SSLKey         string   `mapstructure:"ssl_key" json:"ssl_key" yaml:"ssl_key"`                            // [Optional] SSL Key content or file path
	CACerts        []string `mapstructure:"ca_certs" json:"ca_certs" yaml:"ca_certs"`                         // [Optional] CA Certificates content or file path for verifying client certificates, only used when SSL is enabled
	ClientAuthType string   `mapstructure:"client_auth_type" json:"client_auth_type" yaml:"client_auth_type"` // [Optional] Client authentication type, can be "none", "optional", "required", "must"
}

func (c *Config) IsSSL() bool {
	return c.SSLCert != "" && c.SSLKey != ""
}
