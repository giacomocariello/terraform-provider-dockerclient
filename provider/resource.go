package provider

import (
	"fmt"
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
			"registry": {
				Type:     schema.TypeString,
				Required: true,
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
				Required: true,
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
				Elem:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},

			"all_tags": {
				Type:     schema.TypeList,
				Elem:     schema.TypeString,
				Computed: true,
			},

			"labels": {
				Type:     schema.TypeMap,
				Elem:     schema.TypeString,
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
				Elem:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"ulimit_hard": {
				Type:     schema.TypeMap,
				Elem:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"build_args": {
				Type:     schema.TypeMap,
				Elem:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"auth": {
				Type:      schema.TypeMap,
				Elem:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func resourceDockerImageCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*docker.Client)
	authConfig, err := getAuthConfig(d)
	if err != nil {
		return err
	}

	switch {
	case d.Get("pull").(bool):
		err := client.PullImage(docker.PullImageOptions{
			Repository:        strings.Join([]string{d.Get("registry").(string), d.Get("name").(string)}, ":"),
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int64)) * time.Second,
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
		for ulimitName, ulimitSoft := range d.Get("ulimit_soft").(map[string]int64) {
			ulimit, ok := ulimitMap[ulimitName]
			if !ok {
				ulimit = &docker.ULimit{Name: ulimitName}
				ulimitMap[ulimitName] = ulimit
			}
			ulimit.Soft = ulimitSoft
		}
		for ulimitName, ulimitHard := range d.Get("ulimit_hard").(map[string]int64) {
			ulimit, ok := ulimitMap[ulimitName]
			if !ok {
				ulimit = &docker.ULimit{Name: ulimitName}
				ulimitMap[ulimitName] = ulimit
			}
			ulimit.Hard = ulimitHard
		}
		var ulimitList []docker.ULimit
		for _, ulimit := range ulimitMap {
			ulimitList = append(ulimitList, *ulimit)
		}
		var buildArgList []docker.BuildArg
		for k, v := range d.Get("build_args").(map[string]string) {
			buildArgList = append(buildArgList, docker.BuildArg{
				Name:  k,
				Value: v,
			})
		}
		imageName := strings.Join([]string{d.Get("name").(string), d.Get("tag").(string)}, ":")
		if d.Get("registry").(string) != "" {
			imageName = strings.Join([]string{d.Get("registry").(string), imageName}, ":")
		}
		err := client.BuildImage(docker.BuildImageOptions{
			Name:              imageName,
			Dockerfile:        d.Get("dockerfile").(string),
			SuppressOutput:    true,
			NoCache:           d.Get("nocache").(bool),
			Pull:              d.Get("pull").(bool),
			Memory:            d.Get("memory").(int64),
			Memswap:           d.Get("memswap").(int64),
			CPUShares:         d.Get("cpushares").(int64),
			CPUQuota:          d.Get("cpuquota").(int64),
			CPUPeriod:         d.Get("cpuperiod").(int64),
			CPUSetCPUs:        d.Get("cpusetcpus").(string),
			NetworkMode:       d.Get("networkmode").(string),
			CgroupParent:      d.Get("cgroupparent").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int64)) * time.Second,
			Labels:            d.Get("labels").(map[string]string),
			Remote:            d.Get("build_remote_path").(string),
			ContextDir:        d.Get("build_local_path").(string),
			AuthConfigs: docker.AuthConfigurations{
				Configs: authConfig,
			},
			Ulimits:   ulimitList,
			BuildArgs: buildArgList,
		})
		if err != nil {
			return err
		}
	}

	if d.Get("push").(bool) {
		err := client.PushImage(docker.PushImageOptions{
			Name:              d.Get("name").(string),
			Registry:          d.Get("registry").(string),
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int64)) * time.Second,
		}, authConfig[d.Get("registry").(string)])
		if err != nil {
			return err
		}
	}

	return resourceDockerImageRead(d, meta)
}

func resourceDockerImageRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*docker.Client)

	imageName := strings.Join([]string{d.Get("name").(string), d.Get("tag").(string)}, ":")
	if d.Get("registry").(string) != "" {
		imageName = strings.Join([]string{d.Get("registry").(string), imageName}, ":")
	}

	image, err := client.InspectImage(imageName)
	if err != nil {
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
	authData := d.Get("auth").(map[string]string)
	for authAddress, authPassword := range authData {
		p := strings.SplitN(authAddress, "@", 2)
		if len(p) < 2 {
			return nil, fmt.Errorf("Invalid value for field \"auth\"")
		}
		authHostname, authUsername := p[1], p[0]
		authConfig[authHostname] = docker.AuthConfiguration{
			Username:      authUsername,
			Password:      authPassword,
			ServerAddress: authHostname,
		}
	}
	return authConfig, nil
}

func resourceDockerImageUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*docker.Client)

	authConfig, err := getAuthConfig(d)
	if err != nil {
		return err
	}

	if d.HasChange("push") && d.Get("push").(bool) {
		err := client.PushImage(docker.PushImageOptions{
			Name:              d.Get("name").(string),
			Registry:          d.Get("registry").(string),
			Tag:               d.Get("tag").(string),
			InactivityTimeout: time.Duration(d.Get("timeout").(int64)) * time.Second,
		}, authConfig[d.Get("registry").(string)])
		if err != nil {
			return err
		}
	}
	return nil
}

func resourceDockerImageDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*docker.Client)

	if d.Get("keep").(bool) {
		return nil
	}

	imageName := strings.Join([]string{d.Get("name").(string), d.Get("tag").(string)}, ":")
	if d.Get("registry").(string) != "" {
		imageName = strings.Join([]string{d.Get("registry").(string), imageName}, ":")
	}

	return client.RemoveImageExtended(imageName, docker.RemoveImageOptions{
		Force: true,
	})
}

func resourceDockerImageExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*docker.Client)

	imageName := strings.Join([]string{d.Get("name").(string), d.Get("tag").(string)}, ":")
	if d.Get("registry").(string) != "" {
		imageName = strings.Join([]string{d.Get("registry").(string), imageName}, ":")
	}

	_, err := client.InspectImage(imageName)
	switch err {
	case nil:
		return true, nil
	case docker.ErrNoSuchImage:
		return false, nil
	default:
		return false, err
	}
}
