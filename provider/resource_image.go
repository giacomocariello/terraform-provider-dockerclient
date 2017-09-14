package provider

import (
	"bytes"
	"log"
	"os"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceDockerImage() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerImageCreate,
		Read:   resourceDockerImageRead,
		Update: resourceDockerImageUpdate,
		Delete: resourceDockerImageDelete,
		Exists: resourceDockerImageExists,

		Schema: map[string]*schema.Schema{
			"host": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"machine_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"cert_path": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},

			"ca_material": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},

			"cert_material": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},

			"key_material": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
                                Sensitive:true,
			},

			"registry": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"tag": {
				Type:     schema.TypeString,
				Default:  "latest",
				Optional: true,
				ForceNew: true,
			},

			"build_local_path": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"build_remote_path", "load_path", "pull"},
			},

			"build_remote_path": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"build_local_path", "load_path", "pull"},
			},

			"load_path": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"build_local_path", "build_remote_path", "pull"},
			},

			"pull": {
				Type:          schema.TypeBool,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"build_local_path", "build_remote_path", "load_path"},
			},

			"keep": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"push": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"nocache": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"dockerfile": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"id": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},

			"created_at": {
				Type:     schema.TypeInt,
				Computed: true,
				ForceNew: true,
			},

			"docker_version": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"comment": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"author": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"os": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},

			"architecture": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},

			"size": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"virtual_size": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"parent": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"digests": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
				ForceNew: true,
			},

			"all_tags": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},

			"labels": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},

			"memory": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"memswap": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"cpu_shares": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"cpu_quota": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"cpu_period": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"cpu_set_cpus": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"networkmode": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"cgroup_parent": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"timeout": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"ulimit_soft": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeInt},
				Optional: true,
				ForceNew: true,
			},

			"ulimit_hard": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeInt},
				Optional: true,
				ForceNew: true,
			},

			"build_args": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},

			"auth": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"registry": {
							Type:     schema.TypeString,
							Required: true,
						},
						"username": {
							Type:     schema.TypeString,
							Required: true,
						},
						"password": {
							Type:      schema.TypeString,
							Required:  true,
							Sensitive: true,
						},
					},
				},
				Optional: true,
			},
		},
	}
}

func resourceDockerImageCreate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}
	authConfig, err := getAuthConfig(d)
	if err != nil {
		return err
	}

	repoName := d.Get("name").(string)
	if d.Get("registry").(string) != "" {
		repoName = strings.Join([]string{d.Get("registry").(string), repoName}, "/")
	}

	imageName := repoName
	if d.Get("tag").(string) != "" {
		imageName = strings.Join([]string{imageName, d.Get("tag").(string)}, ":")
	}

	switch {
	case d.Get("pull").(bool):
		err := client.PullImage(docker.PullImageOptions{
			Repository:        repoName,
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int)) * time.Second,
		}, authConfig[d.Get("registry").(string)])
		if err != nil {
			return err
		}
	case d.Get("load_path").(string) != "":
		fh, err := os.OpenFile(d.Get("load_path").(string), os.O_RDONLY, 0600)
		if err != nil {
			return err
		}
		defer fh.Close()
		err = client.LoadImage(docker.LoadImageOptions{
			InputStream: fh,
		})
		if err != nil {
			return err
		}
	case d.Get("build_local_path").(string) != "" || d.Get("build_remote_path").(string) != "":
		ulimitMap := make(map[string]*docker.ULimit)
		for ulimitName, ulimitSoft := range d.Get("ulimit_soft").(map[string]interface{}) {
			ulimit, ok := ulimitMap[ulimitName]
			if !ok {
				ulimit = &docker.ULimit{Name: ulimitName}
				ulimitMap[ulimitName] = ulimit
			}
			ulimit.Soft = ulimitSoft.(int64)
		}
		for ulimitName, ulimitHard := range d.Get("ulimit_hard").(map[string]interface{}) {
			ulimit, ok := ulimitMap[ulimitName]
			if !ok {
				ulimit = &docker.ULimit{Name: ulimitName}
				ulimitMap[ulimitName] = ulimit
			}
			ulimit.Hard = ulimitHard.(int64)
		}
		var ulimitList []docker.ULimit
		for _, ulimit := range ulimitMap {
			ulimitList = append(ulimitList, *ulimit)
		}
		var buildArgList []docker.BuildArg
		for k, v := range d.Get("build_args").(map[string]interface{}) {
			buildArgList = append(buildArgList, docker.BuildArg{
				Name:  k,
				Value: v.(string),
			})
		}

		labelMap := make(map[string]string)
		for k, v := range d.Get("labels").(map[string]interface{}) {
			labelMap[k] = v.(string)
		}

		buf := new(bytes.Buffer)

		err := client.BuildImage(docker.BuildImageOptions{
			Name:              imageName,
			Dockerfile:        d.Get("dockerfile").(string),
			SuppressOutput:    false,
			OutputStream:      buf,
			NoCache:           d.Get("nocache").(bool),
			Pull:              d.Get("pull").(bool),
			Memory:            int64(d.Get("memory").(int)),
			Memswap:           int64(d.Get("memswap").(int)),
			CPUShares:         int64(d.Get("cpu_shares").(int)),
			CPUQuota:          int64(d.Get("cpu_quota").(int)),
			CPUPeriod:         int64(d.Get("cpu_period").(int)),
			CPUSetCPUs:        d.Get("cpu_set_cpus").(string),
			NetworkMode:       d.Get("networkmode").(string),
			CgroupParent:      d.Get("cgroup_parent").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int)) * time.Second,
			Labels:            labelMap,
			Remote:            d.Get("build_remote_path").(string),
			ContextDir:        d.Get("build_local_path").(string),
			AuthConfigs: docker.AuthConfigurations{
				Configs: authConfig,
			},
			Ulimits:   ulimitList,
			BuildArgs: buildArgList,
		})
		log.Printf("docker build command output: %s\n", buf.String())
		if err != nil {
			return err
		}
	}

	if d.Get("push").(bool) {
		err := client.PushImage(docker.PushImageOptions{
			Name:              strings.Join([]string{d.Get("registry").(string), d.Get("name").(string)}, "/"),
			Registry:          d.Get("registry").(string),
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int)) * time.Second,
		}, authConfig[d.Get("registry").(string)])
		if err != nil {
			return err
		}
	}

	d.SetId(imageName)
	return resourceDockerImageRead(d, meta)
}

func resourceDockerImageRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	image, err := client.InspectImage(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}
	d.Set("id", image.ID)
	d.Set("parent", image.Parent)
	d.Set("comment", image.Comment)
	d.Set("docker_version", image.DockerVersion)
	d.Set("author", image.Author)
	d.Set("architecture", image.Architecture)
	d.Set("size", image.Size)
	d.Set("virtual_size", image.VirtualSize)
	d.Set("os", image.OS)
	d.Set("created_at", image.Created.Unix())
	d.Set("labels", image.Config.Labels)
	d.Set("digests", image.RepoDigests)
	d.Set("all_tags", image.RepoTags)
	return nil
}

func getAuthConfig(d *schema.ResourceData) (map[string]docker.AuthConfiguration, error) {
	authConfig := make(map[string]docker.AuthConfiguration)
	authList := d.Get("auth").([]interface{})
	for _, authEntryIf := range authList {
		authEntry := authEntryIf.(map[string]interface{})
		authConfig[authEntry["registry"].(string)] = docker.AuthConfiguration{
			Username: authEntry["username"].(string),
			Password: authEntry["password"].(string),
		}
	}
	return authConfig, nil
}

func resourceDockerImageUpdate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	authConfig, err := getAuthConfig(d)
	if err != nil {
		return err
	}

	if d.HasChange("push") && d.Get("push").(bool) {
		err := client.PushImage(docker.PushImageOptions{
			Name:              strings.Join([]string{d.Get("registry").(string), d.Get("name").(string)}, "/"),
			Registry:          d.Get("registry").(string),
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int)) * time.Second,
		}, authConfig[d.Get("registry").(string)])
		if err != nil {
			return err
		}
	}
	return nil
}

func resourceDockerImageDelete(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	if d.Get("keep").(bool) {
		return nil
	}

	imageName := strings.Join([]string{d.Get("name").(string), d.Get("tag").(string)}, ":")
	if d.Get("registry").(string) != "" {
		imageName = strings.Join([]string{d.Get("registry").(string), imageName}, "/")
	}

	return client.RemoveImageExtended(imageName, docker.RemoveImageOptions{
		Force: true,
	})
}

func resourceDockerImageExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, deferred, err := providerConfig.GetResolvedConfig(d)
	if deferred {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return false, err
	}

	_, err = client.InspectImage(d.Id())
	switch err {
	case nil:
		return true, nil
	case docker.ErrNoSuchImage:
		return false, nil
	default:
		return false, err
	}
}
