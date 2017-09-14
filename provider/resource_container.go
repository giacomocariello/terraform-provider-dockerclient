// Implementation of resourceDockerContainer is derived from github.com/terraform-providers/terraform-provider-docker.
// See https://github.com/terraform-providers/terraform-provider-docker/blob/master/LICENSE for original licensing details.

package provider

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"

	dc "github.com/fsouza/go-dockerclient"
)

var (
	creationTime time.Time
)

func resourceDockerContainer() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerContainerCreate,
		Read:   resourceDockerContainerRead,
		Update: resourceDockerContainerUpdate,
		Delete: resourceDockerContainerDelete,
		Exists: resourceDockerContainerExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

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

			// Indicates whether the container must be running.
			//
			// An assumption is made that configured containers
			// should be running; if not, they should not be in
			// the configuration. Therefore a stopped container
			// should be started. Set to false to have the
			// provider leave the container alone.
			//
			// Actively-debugged containers are likely to be
			// stopped and started manually, and Docker has
			// some provisions for restarting containers that
			// stop. The utility here comes from the fact that
			// this will delete and re-create the container
			// following the principle that the containers
			// should be pristine when started.
			"must_run": {
				Type:     schema.TypeBool,
				Default:  true,
				Optional: true,
			},

			// ForceNew is not true for image because we need to
			// sane this against Docker image IDs, as each image
			// can have multiple names/tags attached do it.
			"image": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"hostname": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"domainname": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"command": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"entrypoint": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"user": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"dns": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"dns_opts": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"dns_search": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"publish_all_ports": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"restart": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "no",
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(string)
					if !regexp.MustCompile(`^(no|on-failure|always|unless-stopped)$`).MatchString(value) {
						es = append(es, fmt.Errorf(
							"%q must be one of \"no\", \"on-failure\", \"always\" or \"unless-stopped\"", k))
					}
					return
				},
			},

			"max_retry_count": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"capabilities": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"add": {
							Type:     schema.TypeSet,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      schema.HashString,
						},

						"drop": {
							Type:     schema.TypeSet,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      schema.HashString,
						},
					},
				},
				Set: resourceDockerCapabilitiesHash,
			},

			"volumes": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"from_container": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},

						"container_path": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},

						"host_path": {
							Type:         schema.TypeString,
							Optional:     true,
							ForceNew:     true,
							ValidateFunc: validateDockerContainerPath,
						},

						"volume_name": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},

						"read_only": {
							Type:     schema.TypeBool,
							Optional: true,
							ForceNew: true,
						},
					},
				},
				Set: resourceDockerVolumesHash,
			},

			"ports": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"internal": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: true,
						},

						"external": {
							Type:     schema.TypeInt,
							Optional: true,
							ForceNew: true,
						},

						"ip": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},

						"protocol": {
							Type:     schema.TypeString,
							Default:  "tcp",
							Optional: true,
							ForceNew: true,
						},
					},
				},
				Set: resourceDockerPortsHash,
			},

			"extra_hosts": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},

						"host": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
				Set: resourceDockerHostsHash,
			},

			"env": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"links": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"ip_address": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"ip_prefix_length": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"gateway": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"bridge": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"privileged": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"destroy_grace_seconds": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},

			"memory": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(int)
					if value < 0 {
						es = append(es, fmt.Errorf("%q must be greater than or equal to 0", k))
					}
					return
				},
			},

			"memory_swap": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(int)
					if value < -1 {
						es = append(es, fmt.Errorf("%q must be greater than or equal to -1", k))
					}
					return
				},
			},

			"cpu_shares": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(int)
					if value < 0 {
						es = append(es, fmt.Errorf("%q must be greater than or equal to 0", k))
					}
					return
				},
			},

			"log_driver": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "json-file",
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(string)
					if !regexp.MustCompile(`^(json-file|syslog|journald|gelf|fluentd)$`).MatchString(value) {
						es = append(es, fmt.Errorf(
							"%q must be one of \"json-file\", \"syslog\", \"journald\", \"gelf\", or \"fluentd\"", k))
					}
					return
				},
			},

			"log_opts": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},

			"network_alias": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"network_mode": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"networks": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"upload": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"content": {
							Type:     schema.TypeString,
							Required: true,
							// This is intentional. The container is mutated once, and never updated later.
							// New configuration forces a new deployment, even with the same binaries.
							ForceNew: true,
						},
						"file": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
				Set: resourceDockerUploadHash,
			},
		},
	}
}

func resourceDockerCapabilitiesHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["add"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v))
	}

	if v, ok := m["remove"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v))
	}

	return hashcode.String(buf.String())
}

func resourceDockerPortsHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	buf.WriteString(fmt.Sprintf("%v-", m["internal"].(int)))

	if v, ok := m["external"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(int)))
	}

	if v, ok := m["ip"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["protocol"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	return hashcode.String(buf.String())
}

func resourceDockerHostsHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["ip"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["host"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	return hashcode.String(buf.String())
}

func resourceDockerVolumesHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["from_container"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["container_path"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["host_path"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["volume_name"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["read_only"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(bool)))
	}

	return hashcode.String(buf.String())
}

func resourceDockerUploadHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["content"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["file"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	return hashcode.String(buf.String())
}

func validateDockerContainerPath(v interface{}, k string) (ws []string, errors []error) {

	value := v.(string)
	if !regexp.MustCompile(`^[a-zA-Z]:\\|^/`).MatchString(value) {
		errors = append(errors, fmt.Errorf("%q must be an absolute path", k))
	}

	return
}

func resourceDockerContainerCreate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	var data Data
	if err := fetchLocalImages(&data, client); err != nil {
		return err
	}

	image := d.Get("image").(string)
	if _, ok := data.DockerImages[image]; !ok {
		if _, ok := data.DockerImages[image+":latest"]; !ok {
			return fmt.Errorf("Unable to find image %s", image)
		}
		image = image + ":latest"
	}

	// The awesome, wonderful, splendiferous, sensical
	// Docker API now lets you specify a HostConfig in
	// CreateContainerOptions, but in my testing it still only
	// actually applies HostConfig options set in StartContainer.
	// How cool is that?
	createOpts := dc.CreateContainerOptions{
		Name: d.Get("name").(string),
		Config: &dc.Config{
			Image:      image,
			Hostname:   d.Get("hostname").(string),
			Domainname: d.Get("domainname").(string),
		},
	}

	if v, ok := d.GetOk("env"); ok {
		createOpts.Config.Env = stringSetToStringSlice(v.(*schema.Set))
	}

	if v, ok := d.GetOk("command"); ok {
		createOpts.Config.Cmd = stringListToStringSlice(v.([]interface{}))
		for _, v := range createOpts.Config.Cmd {
			if v == "" {
				return fmt.Errorf("values for command may not be empty")
			}
		}
	}

	if v, ok := d.GetOk("entrypoint"); ok {
		createOpts.Config.Entrypoint = stringListToStringSlice(v.([]interface{}))
	}

	if v, ok := d.GetOk("user"); ok {
		createOpts.Config.User = v.(string)
	}

	exposedPorts := map[dc.Port]struct{}{}
	portBindings := map[dc.Port][]dc.PortBinding{}

	if v, ok := d.GetOk("ports"); ok {
		exposedPorts, portBindings = portSetToDockerPorts(v.(*schema.Set))
	}
	if len(exposedPorts) != 0 {
		createOpts.Config.ExposedPorts = exposedPorts
	}

	extraHosts := []string{}
	if v, ok := d.GetOk("extra_hosts"); ok {
		extraHosts = extraHostsSetToDockerExtraHosts(v.(*schema.Set))
	}

	volumes := map[string]struct{}{}
	binds := []string{}
	volumesFrom := []string{}

	if v, ok := d.GetOk("volumes"); ok {
		volumes, binds, volumesFrom, err = volumeSetToDockerVolumes(v.(*schema.Set))
		if err != nil {
			return fmt.Errorf("Unable to parse volumes: %s", err)
		}
	}
	if len(volumes) != 0 {
		createOpts.Config.Volumes = volumes
	}

	if v, ok := d.GetOk("labels"); ok {
		createOpts.Config.Labels = mapTypeMapValsToString(v.(map[string]interface{}))
	}

	hostConfig := &dc.HostConfig{
		Privileged:      d.Get("privileged").(bool),
		PublishAllPorts: d.Get("publish_all_ports").(bool),
		RestartPolicy: dc.RestartPolicy{
			Name:              d.Get("restart").(string),
			MaximumRetryCount: d.Get("max_retry_count").(int),
		},
		LogConfig: dc.LogConfig{
			Type: d.Get("log_driver").(string),
		},
	}

	if len(portBindings) != 0 {
		hostConfig.PortBindings = portBindings
	}
	if len(extraHosts) != 0 {
		hostConfig.ExtraHosts = extraHosts
	}
	if len(binds) != 0 {
		hostConfig.Binds = binds
	}
	if len(volumesFrom) != 0 {
		hostConfig.VolumesFrom = volumesFrom
	}

	if v, ok := d.GetOk("capabilities"); ok {
		for _, capInt := range v.(*schema.Set).List() {
			capa := capInt.(map[string]interface{})
			hostConfig.CapAdd = stringSetToStringSlice(capa["add"].(*schema.Set))
			hostConfig.CapDrop = stringSetToStringSlice(capa["drop"].(*schema.Set))
			break
		}
	}

	if v, ok := d.GetOk("dns"); ok {
		hostConfig.DNS = stringSetToStringSlice(v.(*schema.Set))
	}

	if v, ok := d.GetOk("dns_opts"); ok {
		hostConfig.DNSOptions = stringSetToStringSlice(v.(*schema.Set))
	}

	if v, ok := d.GetOk("dns_search"); ok {
		hostConfig.DNSSearch = stringSetToStringSlice(v.(*schema.Set))
	}

	if v, ok := d.GetOk("links"); ok {
		hostConfig.Links = stringSetToStringSlice(v.(*schema.Set))
	}

	if v, ok := d.GetOk("memory"); ok {
		hostConfig.Memory = int64(v.(int)) * 1024 * 1024
	}

	if v, ok := d.GetOk("memory_swap"); ok {
		swap := int64(v.(int))
		if swap > 0 {
			swap = swap * 1024 * 1024
		}
		hostConfig.MemorySwap = swap
	}

	if v, ok := d.GetOk("cpu_shares"); ok {
		hostConfig.CPUShares = int64(v.(int))
	}

	if v, ok := d.GetOk("log_opts"); ok {
		hostConfig.LogConfig.Config = mapTypeMapValsToString(v.(map[string]interface{}))
	}

	if v, ok := d.GetOk("network_mode"); ok {
		hostConfig.NetworkMode = v.(string)
	}

	createOpts.HostConfig = hostConfig

	var retContainer *dc.Container
	if retContainer, err = client.CreateContainer(createOpts); err != nil {
		return fmt.Errorf("Unable to create container: %s", err)
	}
	if retContainer == nil {
		return fmt.Errorf("Returned container is nil")
	}

	d.SetId(retContainer.ID)

	if v, ok := d.GetOk("networks"); ok {
		var connectionOpts dc.NetworkConnectionOptions
		if v, ok := d.GetOk("network_alias"); ok {
			endpointConfig := &dc.EndpointConfig{}
			endpointConfig.Aliases = stringSetToStringSlice(v.(*schema.Set))
			connectionOpts = dc.NetworkConnectionOptions{Container: retContainer.ID, EndpointConfig: endpointConfig}
		} else {
			connectionOpts = dc.NetworkConnectionOptions{Container: retContainer.ID}
		}

		for _, rawNetwork := range v.(*schema.Set).List() {
			network := rawNetwork.(string)
			if err := client.ConnectNetwork(network, connectionOpts); err != nil {
				return fmt.Errorf("Unable to connect to network '%s': %s", network, err)
			}
		}
	}

	if v, ok := d.GetOk("upload"); ok {
		for _, upload := range v.(*schema.Set).List() {
			content := upload.(map[string]interface{})["content"].(string)
			file := upload.(map[string]interface{})["file"].(string)

			buf := new(bytes.Buffer)
			tw := tar.NewWriter(buf)
			hdr := &tar.Header{
				Name: file,
				Mode: 0644,
				Size: int64(len(content)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("Error creating tar archive: %s", err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				return fmt.Errorf("Error creating tar archive: %s", err)
			}
			if err := tw.Close(); err != nil {
				return fmt.Errorf("Error creating tar archive: %s", err)
			}

			uploadOpts := dc.UploadToContainerOptions{
				InputStream: bytes.NewReader(buf.Bytes()),
				Path:        "/",
			}

			if err := client.UploadToContainer(retContainer.ID, uploadOpts); err != nil {
				return fmt.Errorf("Unable to upload volume content: %s", err)
			}
		}
	}

	creationTime = time.Now()
	if err := client.StartContainer(retContainer.ID, nil); err != nil {
		return fmt.Errorf("Unable to start container: %s", err)
	}

	return resourceDockerContainerRead(d, meta)
}

func resourceDockerContainerRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	apiContainer, err := fetchDockerContainer(d.Id(), client)
	if err != nil {
		return err
	}
	if apiContainer == nil {
		// This container doesn't exist anymore
		d.SetId("")
		return nil
	}

	var container *dc.Container

	loops := 1 // if it hasn't just been created, don't delay
	if !creationTime.IsZero() {
		loops = 30 // with 500ms spacing, 15 seconds; ought to be plenty
	}
	sleepTime := 500 * time.Millisecond

	for i := loops; i > 0; i-- {
		container, err = client.InspectContainer(apiContainer.ID)
		if err != nil {
			return fmt.Errorf("Error inspecting container %s: %s", apiContainer.ID, err)
		}

		if container.State.Running ||
			!container.State.Running && !d.Get("must_run").(bool) {
			break
		}

		if creationTime.IsZero() { // We didn't just create it, so don't wait around
			return resourceDockerContainerDelete(d, meta)
		}

		if container.State.FinishedAt.After(creationTime) {
			// It exited immediately, so error out so dependent containers
			// aren't started
			resourceDockerContainerDelete(d, meta)
			return fmt.Errorf("Container %s exited after creation, error was: %s", apiContainer.ID, container.State.Error)
		}

		time.Sleep(sleepTime)
	}

	// Handle the case of the for loop above running its course
	if !container.State.Running && d.Get("must_run").(bool) {
		resourceDockerContainerDelete(d, meta)
		return fmt.Errorf("Container %s failed to be in running state", apiContainer.ID)
	}

	// Read Network Settings
	if container.NetworkSettings != nil {
		d.Set("ip_address", container.NetworkSettings.IPAddress)
		d.Set("ip_prefix_length", container.NetworkSettings.IPPrefixLen)
		d.Set("gateway", container.NetworkSettings.Gateway)
		d.Set("bridge", container.NetworkSettings.Bridge)
	}

	return nil
}

func resourceDockerContainerUpdate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceDockerContainerDelete(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	// Stop the container before removing if destroy_grace_seconds is defined
	if d.Get("destroy_grace_seconds").(int) > 0 {
		var timeout = uint(d.Get("destroy_grace_seconds").(int))
		if err := client.StopContainer(d.Id(), timeout); err != nil {
			return fmt.Errorf("Error stopping container %s: %s", d.Id(), err)
		}
	}

	removeOpts := dc.RemoveContainerOptions{
		ID:            d.Id(),
		RemoveVolumes: true,
		Force:         true,
	}

	if err := client.RemoveContainer(removeOpts); err != nil {
		return fmt.Errorf("Error deleting container %s: %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func resourceDockerContainerExists(d *schema.ResourceData, meta interface{}) (bool, error) {
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

	apiContainer, err := fetchDockerContainer(d.Id(), client)
	if err != nil {
		return false, err
	}
	if apiContainer == nil {
		return false, nil
	}

	return true, nil
}

func stringListToStringSlice(stringList []interface{}) []string {
	ret := []string{}
	for _, v := range stringList {
		if v == nil {
			ret = append(ret, "")
			continue
		}
		ret = append(ret, v.(string))
	}
	return ret
}

func stringSetToStringSlice(stringSet *schema.Set) []string {
	ret := []string{}
	if stringSet == nil {
		return ret
	}
	for _, envVal := range stringSet.List() {
		ret = append(ret, envVal.(string))
	}
	return ret
}

func mapTypeMapValsToString(typeMap map[string]interface{}) map[string]string {
	mapped := make(map[string]string, len(typeMap))
	for k, v := range typeMap {
		mapped[k] = v.(string)
	}
	return mapped
}

func fetchDockerContainer(ID string, client *dc.Client) (*dc.APIContainers, error) {
	apiContainers, err := client.ListContainers(dc.ListContainersOptions{All: true})

	if err != nil {
		return nil, fmt.Errorf("Error fetching container information from Docker: %s\n", err)
	}

	for _, apiContainer := range apiContainers {
		if apiContainer.ID == ID {
			return &apiContainer, nil
		}
	}

	return nil, nil
}

func portSetToDockerPorts(ports *schema.Set) (map[dc.Port]struct{}, map[dc.Port][]dc.PortBinding) {
	retExposedPorts := map[dc.Port]struct{}{}
	retPortBindings := map[dc.Port][]dc.PortBinding{}

	for _, portInt := range ports.List() {
		port := portInt.(map[string]interface{})
		internal := port["internal"].(int)
		protocol := port["protocol"].(string)

		exposedPort := dc.Port(strconv.Itoa(internal) + "/" + protocol)
		retExposedPorts[exposedPort] = struct{}{}

		external, extOk := port["external"].(int)
		ip, ipOk := port["ip"].(string)

		if extOk {
			portBinding := dc.PortBinding{
				HostPort: strconv.Itoa(external),
			}
			if ipOk {
				portBinding.HostIP = ip
			}
			retPortBindings[exposedPort] = append(retPortBindings[exposedPort], portBinding)
		}
	}

	return retExposedPorts, retPortBindings
}

func extraHostsSetToDockerExtraHosts(extraHosts *schema.Set) []string {
	retExtraHosts := []string{}

	for _, hostInt := range extraHosts.List() {
		host := hostInt.(map[string]interface{})
		ip := host["ip"].(string)
		hostname := host["host"].(string)
		retExtraHosts = append(retExtraHosts, hostname+":"+ip)
	}

	return retExtraHosts
}

func volumeSetToDockerVolumes(volumes *schema.Set) (map[string]struct{}, []string, []string, error) {
	retVolumeMap := map[string]struct{}{}
	retHostConfigBinds := []string{}
	retVolumeFromContainers := []string{}

	for _, volumeInt := range volumes.List() {
		volume := volumeInt.(map[string]interface{})
		fromContainer := volume["from_container"].(string)
		containerPath := volume["container_path"].(string)
		volumeName := volume["volume_name"].(string)
		if len(volumeName) == 0 {
			volumeName = volume["host_path"].(string)
		}
		readOnly := volume["read_only"].(bool)

		switch {
		case len(fromContainer) == 0 && len(containerPath) == 0:
			return retVolumeMap, retHostConfigBinds, retVolumeFromContainers, errors.New("Volume entry without container path or source container")
		case len(fromContainer) != 0 && len(containerPath) != 0:
			return retVolumeMap, retHostConfigBinds, retVolumeFromContainers, errors.New("Both a container and a path specified in a volume entry")
		case len(fromContainer) != 0:
			retVolumeFromContainers = append(retVolumeFromContainers, fromContainer)
		case len(volumeName) != 0:
			readWrite := "rw"
			if readOnly {
				readWrite = "ro"
			}
			retVolumeMap[containerPath] = struct{}{}
			retHostConfigBinds = append(retHostConfigBinds, volumeName+":"+containerPath+":"+readWrite)
		default:
			retVolumeMap[containerPath] = struct{}{}
		}
	}

	return retVolumeMap, retHostConfigBinds, retVolumeFromContainers, nil
}

func fetchLocalImages(data *Data, client *dc.Client) error {
	images, err := client.ListImages(dc.ListImagesOptions{All: false})
	if err != nil {
		return fmt.Errorf("Unable to list Docker images: %s", err)
	}

	if data.DockerImages == nil {
		data.DockerImages = make(map[string]*dc.APIImages)
	}

	// Docker uses different nomenclatures in different places...sometimes a short
	// ID, sometimes long, etc. So we store both in the map so we can always find
	// the same image object. We store the tags, too.
	for i, image := range images {
		data.DockerImages[image.ID[:12]] = &images[i]
		data.DockerImages[image.ID] = &images[i]
		for _, repotag := range image.RepoTags {
			data.DockerImages[repotag] = &images[i]
		}
	}

	return nil
}
