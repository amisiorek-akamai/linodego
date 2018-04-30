package golinode

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-resty/resty"
)

/*
 * https://developers.linode.com/v4/reference/endpoints/linode/instances
 */

// Instance represents a linode object
type Instance struct {
	CreatedStr string `json:"created"`
	UpdatedStr string `json:"updated"`

	ID         int
	Created    *time.Time `json:"-"`
	Updated    *time.Time `json:"-"`
	Region     string
	Alerts     *InstanceAlert
	Backups    *InstanceBackup
	Image      string
	Group      string
	IPv4       []*net.IP
	IPv6       string
	Label      string
	Type       string
	Status     string
	Hypervisor string
	Specs      *InstanceSpec
}

// InstanceSpec represents a linode spec
type InstanceSpec struct {
	Disk     int
	Memory   int
	VCPUs    int
	Transfer int
}

// InstanceAlert represents a metric alert
type InstanceAlert struct {
	CPU           int
	IO            int
	NetworkIn     int
	NetworkOut    int
	TransferQuote int
}

// InstanceBackup represents backup settings for an instance
type InstanceBackup struct {
	Enabled  bool
	Schedule struct {
		Day    string
		Window string
	}
}

// InstanceCreateOptions require only Region and Type
type InstanceCreateOptions struct {
	Region          string            `json:"region"`
	Type            string            `json:"type"`
	Label           string            `json:"label,omitempty"`
	Group           string            `json:"group,omitempty"`
	RootPass        string            `json:"root_pass,omitempty"`
	AuthorizedKeys  []string          `json:"authorized_keys,omitempty"`
	StackScriptID   int               `json:"stackscript_id,omitempty"`
	StackScriptData map[string]string `json:"stackscript_data,omitempty"`
	BackupID        int               `json:"backup_id,omitempty"`
	Image           string            `json:"image,omitempty"`
	BackupsEnabled  bool              `json:"backups_enabled,omitempty"`
	Booted          bool              `json:"booted,omitempty"`
}

// InstanceCloneOptions is an options struct when sending a clone request to the API
type InstanceCloneOptions struct {
	Region         string
	Type           string
	LinodeID       int
	Label          string
	Group          string
	BackupsEnabled bool
	Disks          []string
	Configs        []string
}

func (l *Instance) fixDates() *Instance {
	l.Created, _ = parseDates(l.CreatedStr)
	l.Updated, _ = parseDates(l.UpdatedStr)
	return l
}

// InstancesPagedResponse represents a linode API response for listing
type InstancesPagedResponse struct {
	*PageOptions
	Data []*Instance
}

// Endpoint gets the endpoint URL for Instance
func (InstancesPagedResponse) Endpoint(c *Client) string {
	endpoint, err := c.Instances.Endpoint()
	if err != nil {
		panic(err)
	}
	return endpoint
}

// AppendData appends Instances when processing paginated Instance responses
func (resp *InstancesPagedResponse) AppendData(r *InstancesPagedResponse) {
	(*resp).Data = append(resp.Data, r.Data...)
}

// SetResult sets the Resty response type of Instance
func (InstancesPagedResponse) SetResult(r *resty.Request) {
	r.SetResult(InstancesPagedResponse{})
}

// ListInstances lists linode instances
func (c *Client) ListInstances(opts *ListOptions) ([]*Instance, error) {
	e, err := c.Instances.Endpoint()
	if err != nil {
		return nil, err
	}

	req := c.R().SetResult(&InstancesPagedResponse{})

	if opts != nil {
		req.SetQueryParam("page", strconv.Itoa(opts.Page))
	}

	r, err := req.Get(e)
	if err != nil {
		return nil, err
	}

	data := r.Result().(*InstancesPagedResponse).Data
	pages := r.Result().(*InstancesPagedResponse).Pages
	results := r.Result().(*InstancesPagedResponse).Results

	for _, el := range data {
		el.fixDates()
	}

	if opts == nil {
		for page := 2; page <= pages; page = page + 1 {
			next, _ := c.ListInstances(&ListOptions{PageOptions: &PageOptions{Page: page}})
			data = append(data, next...)
		}
	} else {
		opts.Results = results
	}

	return data, nil
}

// GetInstance gets the instance with the provided ID
func (c *Client) GetInstance(linodeID int) (*Instance, error) {
	e, err := c.Instances.Endpoint()
	if err != nil {
		return nil, err
	}
	e = fmt.Sprintf("%s/%d", e, linodeID)
	r, err := c.R().
		SetResult(&Instance{}).
		Get(e)
	if err != nil {
		return nil, err
	}
	return r.Result().(*Instance).fixDates(), nil
}

// CreateInstance creates a Linode instance
func (c *Client) CreateInstance(instance *InstanceCreateOptions) (*Instance, error) {
	var body string
	e, err := c.Instances.Endpoint()
	if err != nil {
		return nil, err
	}

	req := c.R().SetResult(&Instance{})

	if bodyData, err := json.Marshal(instance); err == nil {
		body = string(bodyData)
	} else {
		return nil, err
	}

	r, err := req.
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(e)

	if err != nil {
		return nil, err
	}

	return r.Result().(*Instance).fixDates(), nil
}

// BootInstance will boot a new linode instance
func (c *Client) BootInstance(id int, configID int) (bool, error) {
	bodyStr := ""

	if configID != 0 {
		bodyMap := map[string]string{"config_id": string(configID)}
		bodyJSON, err := json.Marshal(bodyMap)
		if err != nil {
			return false, err
		}
		bodyStr = string(bodyJSON)
	}

	e, err := c.Instances.Endpoint()
	if err != nil {
		return false, err
	}

	e = fmt.Sprintf("%s/%d/boot", e, id)
	r, err := c.R().
		SetHeader("Content-Type", "application/json").
		SetBody(bodyStr).
		Post(e)

	return settleBoolResponseOrError(r, err)
}

// CloneInstance clones a Linode instance
func (c *Client) CloneInstance(id int, options *InstanceCloneOptions) (*Instance, error) {
	var body string
	e, err := c.Instances.Endpoint()
	if err != nil {
		return nil, err
	}
	e = fmt.Sprintf("%s/%d/clone", e, id)

	req := c.R().SetResult(&Instance{})

	if bodyData, err := json.Marshal(options); err == nil {
		body = string(bodyData)
	} else {
		return nil, err
	}

	r, err := req.
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(e)

	if err != nil {
		return nil, err
	}

	return r.Result().(*Instance).fixDates(), nil
}

// RebootInstance reboots a Linode instance
func (c *Client) RebootInstance(id int, configID int) (bool, error) {
	body := fmt.Sprintf("{\"config_id\":\"%d\"}", configID)

	e, err := c.Instances.Endpoint()
	if err != nil {
		return false, err
	}

	e = fmt.Sprintf("%s/%d/reboot", e, id)

	r, err := c.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(e)

	return settleBoolResponseOrError(r, err)
}

// MutateInstance Upgrades a Linode to its next generation.
func (c *Client) MutateInstance(id int) (bool, error) {
	e, err := c.Instances.Endpoint()
	if err != nil {
		return false, err
	}
	e = fmt.Sprintf("%s/%d/mutate", e, id)

	r, err := c.R().Post(e)
	return settleBoolResponseOrError(r, err)
}

// RebuildInstanceOptions is a struct representing the options to send to the rebuild linode endpoint
type RebuildInstanceOptions struct {
	Image           string
	RootPass        string
	AuthorizedKeys  []string
	StackscriptID   int
	StackscriptData map[string]string
	Booted          bool
}

// RebuildInstance Deletes all Disks and Configs on this Linode,
// then deploys a new Image to this Linode with the given attributes.
func (c *Client) RebuildInstance(id int, opts *RebuildInstanceOptions) (*Instance, error) {
	o, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	b := string(o)
	e, err := c.Instances.Endpoint()
	if err != nil {
		return nil, err
	}
	e = fmt.Sprintf("%s/%d/rebuild", e, id)
	r, err := c.R().
		SetHeader("Content-Type", "application/json").
		SetBody(b).
		SetResult(&Instance{}).
		Post(e)
	if err != nil {
		return nil, err
	}
	return r.Result().(*Instance).fixDates(), nil
}

// ResizeInstance resizes an instance to new Linode type
func (c *Client) ResizeInstance(id int, linodeType string) (bool, error) {
	body := fmt.Sprintf("{\"type\":\"%s\"}", linodeType)

	e, err := c.Instances.Endpoint()
	if err != nil {
		return false, err
	}
	e = fmt.Sprintf("%s/%d/resize", e, id)

	r, err := c.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(e)

	return settleBoolResponseOrError(r, err)
}

// ShutdownInstance - Shutdown an instance
func (c *Client) ShutdownInstance(id int) (bool, error) {
	e, err := c.Instances.Endpoint()
	if err != nil {
		return false, err
	}
	e = fmt.Sprintf("%s/%d/resize", e, id)
	return settleBoolResponseOrError(c.R().Post(e))
}

// ListInstanceVolumes lists volumes attached to a linode instance
func (c *Client) ListInstanceVolumes(id int) ([]*Volume, error) {
	e, err := c.Instances.Endpoint()
	e = fmt.Sprintf("%s/%d/volumes", e, id)
	if err != nil {
		return nil, err
	}
	resp, err := c.R().
		SetResult(&VolumesPagedResponse{}).
		Get(e)
	if err != nil {
		return nil, err
	}
	l := resp.Result().(*VolumesPagedResponse).Data
	for _, el := range l {
		el.fixDates()
	}
	return l, nil
}

func settleBoolResponseOrError(resp *resty.Response, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	return true, nil
}
