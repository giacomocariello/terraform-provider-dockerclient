package provider

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

type ProviderConfig struct {
	HostURI      string
	HostName     string
	CaMaterial   []byte
	CertMaterial []byte
	KeyMaterial  []byte
	CaFile       string
	CertFile     string
	KeyFile      string
	CertPath     string
	StoragePath  string
	Ping         bool
}

type ResourceDockerConfig struct {
	HostURI      string
	HostName     string
	CaMaterial   []byte
	CertMaterial []byte
	KeyMaterial  []byte
	Ping         bool
}

type Data struct {
	DockerImages map[string]*dc.APIImages
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"default_host": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOCKER_HOST", "unix:///var/run/docker.sock"),
				Description: "The Docker daemon address",
			},

			"ca_material": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOCKER_CA_MATERIAL", ""),
				Description: "PEM-encoded content of Docker host CA certificate",
			},
			"ca_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded content of Docker host CA certificate",
			},

			"cert_material": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOCKER_CERT_MATERIAL", ""),
				Description: "PEM-encoded content of Docker client certificate",
			},
			"cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded content of Docker client certificate",
			},

			"key_material": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOCKER_KEY_MATERIAL", ""),
				Description: "PEM-encoded content of Docker client private key",
			},
			"key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded content of Docker client private key",
			},

			"cert_path": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOCKER_CERT_PATH", ""),
				Description: "Path to directory with Docker TLS config",
			},

			"storage_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to directory with Docker Machine config",
			},

			"ping": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Ping docker host on connect",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"dockerclient_container": resourceDockerContainer(),
			"dockerclient_image":     resourceDockerImage(),
			"dockerclient_network":   resourceDockerNetwork(),
			"dockerclient_volume":    resourceDockerVolume(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	return &ProviderConfig{
		HostURI: d.Get("default_host").(string),
		HostName: d.Get("default_hostname").(string),

		CaMaterial:   []byte(d.Get("ca_material").(string)),
		CertMaterial: []byte(d.Get("cert_material").(string)),
		KeyMaterial:  []byte(d.Get("key_material").(string)),

		CaFile:   d.Get("ca_file").(string),
		CertFile: d.Get("cert_file").(string),
		KeyFile:  d.Get("key_file").(string),

		CertPath:    d.Get("cert_path").(string),
		StoragePath: d.Get("storage_path").(string),

		Ping: d.Get("ping").(bool),
	}, nil
}

func (c *ProviderConfig) GetResolvedConfig(d *schema.ResourceData) (*ProviderConfig, bool, error) {
	r := &ProviderConfig{
		Ping: c.Ping,
	}
	var certPath string

	if d.Get("host").(string) != "" {
		r.HostURI = d.Get("host").(string)
	} else if c.HostURI != "" {
		r.HostURI = c.HostURI
	} else {
		return nil, true, fmt.Errorf("host is not set")
	}

	if d.Get("hostname").(string) != "" {
		r.HostName = d.Get("hostname").(string)
	} else if c.HostName != "" {
		r.HostName = c.HostName
	} else {
		return nil, true, fmt.Errorf("hostname is not set")
	}

	if (len(c.CaMaterial) == 0 && c.CaFile == "") ||
		(len(c.CertMaterial) == 0 && c.CertFile == "") ||
		(len(c.KeyMaterial) == 0 && c.KeyFile == "") {
		switch {
		case c.CertPath != "":
			certPath = c.CertPath
		case c.StoragePath != "":
			certPath = filepath.Join(c.CertPath, "machines", r.HostName)
		}
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			return nil, true, fmt.Errorf("error trying to stat cert_path: %s", err)
		} else if err != nil {
			return nil, false, fmt.Errorf("error trying to stat cert_path: %s", err)
		}
	}

	if len(c.CaMaterial) != 0 {
		r.CaMaterial = c.CaMaterial
	} else {
		var caFile string
		switch {
		case c.CaFile != "":
			caFile = c.CaFile
		case certPath != "":
			caFile = filepath.Join(certPath, "ca.pem")
		}
		if caFile != "" {
			if _, err := os.Stat(caFile); err == nil {
				r.CaMaterial, err = ioutil.ReadFile(caFile)
				if err != nil {
					return nil, false, fmt.Errorf("error reading ca file: %s", err)
				}
			} else {
				return nil, false, fmt.Errorf("error reading ca file: %s", err)
			}
		}
	}

	if len(c.CertMaterial) != 0 {
		r.CertMaterial = c.CertMaterial
	} else {
		var certFile string
		switch {
		case c.CertFile != "":
			certFile = c.CertFile
		case certPath != "":
			certFile = filepath.Join(certPath, "cert.pem")
		}
		if certFile != "" {
			if _, err := os.Stat(certFile); err == nil {
				r.CertMaterial, err = ioutil.ReadFile(certFile)
				if err != nil {
					return nil, false, fmt.Errorf("error reading cert file: %s", err)
				}
			} else {
				return nil, false, fmt.Errorf("error reading cert file: %s", err)
			}
		}
	}

	if len(c.KeyMaterial) != 0 {
		r.KeyMaterial = c.KeyMaterial
	} else {
		var keyFile string
		switch {
		case c.KeyFile != "":
			keyFile = c.KeyFile
		case certPath != "":
			keyFile = filepath.Join(certPath, "key.pem")
		}
		if keyFile != "" {
			if _, err := os.Stat(keyFile); err == nil {
				r.KeyMaterial, err = ioutil.ReadFile(keyFile)
				if err != nil {
					return nil, false, fmt.Errorf("error reading key file: %s", err)
				}
			} else {
				return nil, false, fmt.Errorf("error reading key file: %s", err)
			}
		}
	}

	if len(c.CaMaterial) > 0 || len(c.CertMaterial) > 0 || len(c.KeyMaterial) > 0 {
		if len(c.CaMaterial) == 0 {
			return nil, false, fmt.Errorf("missing CA certificate")
		}
		if len(c.CertMaterial) == 0 {
			return nil, false, fmt.Errorf("missing client certificate")
		}
		if len(c.KeyMaterial) == 0 {
			return nil, false, fmt.Errorf("missing key certificate")
		}
	}

	return r, false, nil
}

func (c *ProviderConfig) NewClient() (client *dc.Client, err error) {
	if len(c.CertMaterial) != 0 && len(c.KeyMaterial) != 0 && len(c.CaMaterial) != 0 {
		client, err = dc.NewTLSClientFromBytes(c.HostURI, c.CertMaterial, c.KeyMaterial, c.CaMaterial)
	} else {
		client, err = dc.NewClient(c.HostURI)
	}
	if err != nil {
		return nil, fmt.Errorf("error opening docker client: %s", err)
	}
	if c.Ping {
		err = client.Ping()
		if err != nil {
			return nil, fmt.Errorf("error pinging Docker server: %s", err)
		}
	}
	return client, nil

}
