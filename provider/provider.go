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
	Host         string
	CaMaterial   []byte
	CertMaterial []byte
	KeyMaterial  []byte
	CaFile       string
	CertFile     string
	KeyFile      string
	CertPath     string
	Ping         bool
}

type Data struct {
	DockerImages map[string]*dc.APIImages
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
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

			"ping": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Ping docker host on connect",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"dockerimage": resourceDockerImage(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	return &ProviderConfig{
		Host:         d.Get("host").(string),
		CaFile:       d.Get("ca_file").(string),
		CaMaterial:   []byte(d.Get("ca_material").(string)),
		CertMaterial: []byte(d.Get("cert_material").(string)),
		KeyMaterial:  []byte(d.Get("key_material").(string)),
		CertFile:     d.Get("cert_file").(string),
		KeyFile:      d.Get("key_file").(string),
		CertPath:     d.Get("cert_path").(string),
		Ping:         d.Get("ping").(bool),
	}, nil
}

func (c *ProviderConfig) NewClient() (*dc.Client, error) {
	var client *dc.Client
	var err error

	if len(c.CaMaterial) == 0 {
		if c.CaFile == "" && c.CertPath != "" {
			c.CaFile = filepath.Join(c.CertPath, "ca.pem")
		}
		if c.CaFile != "" {
			if _, err := os.Stat(c.CaFile); !os.IsNotExist(err) {
				c.CaMaterial, err = ioutil.ReadFile(c.CaFile)
				if err != nil {
					return nil, fmt.Errorf("error reading ca file: %s", err)
				}
			} else if err != nil {
				return nil, fmt.Errorf("error reading ca file: %s", err)
			}
		}
	}

	if len(c.CertMaterial) == 0 {
		if c.CertFile == "" && c.CertPath != "" {
			c.CertFile = filepath.Join(c.CertPath, "cert.pem")
		}
		if c.CertFile != "" {
			if _, err := os.Stat(c.CertFile); !os.IsNotExist(err) {
				c.CertMaterial, err = ioutil.ReadFile(c.CertFile)
				if err != nil {
					return nil, fmt.Errorf("error reading cert file: %s", err)
				}
			} else if err != nil {
				return nil, fmt.Errorf("error reading cert file: %s", err)
			}
		}
	}

	if len(c.KeyMaterial) == 0 {
		if c.KeyFile == "" && c.CertPath != "" {
			c.KeyFile = filepath.Join(c.CertPath, "key.pem")
		}
		if c.KeyFile != "" {
			if _, err := os.Stat(c.KeyFile); !os.IsNotExist(err) {
				c.KeyMaterial, err = ioutil.ReadFile(c.KeyFile)
				if err != nil {
					return nil, fmt.Errorf("error reading key file: %s", err)
				}
			} else if err != nil {
				return nil, fmt.Errorf("error reading key file: %s", err)
			}
		}
	}

	if len(c.CaMaterial) > 0 || len(c.CertMaterial) > 0 || len(c.KeyMaterial) > 0 {
		if len(c.CaMaterial) == 0 {
			return nil, fmt.Errorf("missing CA certificate")
		}
		if len(c.CertMaterial) == 0 {
			return nil, fmt.Errorf("missing client certificate")
		}
		if len(c.KeyMaterial) == 0 {
			return nil, fmt.Errorf("missing key certificate")
		}

		client, err = dc.NewTLSClientFromBytes(c.Host, c.CertMaterial, c.KeyMaterial, c.CaMaterial)
	} else {
		client, err = dc.NewClient(c.Host)
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
