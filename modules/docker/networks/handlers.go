package networks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/docker/envcli"
	"github.com/izetmolla/containerws/packages/dockerclient"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	list := api.Group("/list")
	list.Get("/", cc.ListAPI)

	single := api.Group("/single")
	single.Post("/", cc.CreateAPI)
	single.Get("/:id", cc.InspectAPI)
	single.Post("/:id/disconnect", cc.DisconnectAPI)
	single.Delete("/:id", cc.RemoveAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

type networkRow struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"short_id"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	Created    string            `json:"created,omitempty"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	Ingress    bool              `json:"ingress"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	items, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]networkRow, 0, len(items))
	for _, it := range items {
		short := it.ID
		if len(short) > 12 {
			short = short[:12]
		}
		created := ""
		if !it.Created.IsZero() {
			created = it.Created.Format(time.RFC3339)
		}
		rows = append(rows, networkRow{
			ID:         it.ID,
			ShortID:    short,
			Name:       it.Name,
			Driver:     it.Driver,
			Scope:      it.Scope,
			Created:    created,
			Internal:   it.Internal,
			Attachable: it.Attachable,
			Ingress:    it.Ingress,
			Labels:     it.Labels,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type ipamBody struct {
	Subnet   string   `json:"subnet"`
	Gateway  string   `json:"gateway"`
	IPRange  string   `json:"ip_range"`
	Excluded []string `json:"excluded_ips"` // aux addresses values
}

type createBody struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Options    map[string]string `json:"options"`
	Internal   bool              `json:"internal"`   // isolated network
	Attachable bool              `json:"attachable"` // manual container attachment
	EnableIPv6 bool              `json:"enable_ipv6"`
	Labels     map[string]string `json:"labels"`
	IPv4       *ipamBody         `json:"ipv4"`
	IPv6       *ipamBody         `json:"ipv6"`
	// Legacy flat fields (still accepted)
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	IPRange string `json:"ip_range"`
}

func ipamFromBody(b *ipamBody) *network.IPAMConfig {
	if b == nil {
		return nil
	}
	subnet := strings.TrimSpace(b.Subnet)
	if subnet == "" {
		return nil
	}
	cfg := &network.IPAMConfig{Subnet: subnet}
	if g := strings.TrimSpace(b.Gateway); g != "" {
		cfg.Gateway = g
	}
	if ir := strings.TrimSpace(b.IPRange); ir != "" {
		cfg.IPRange = ir
	}
	if len(b.Excluded) > 0 {
		aux := map[string]string{}
		for i, ip := range b.Excluded {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			aux[fmt.Sprintf("excluded_%d", i)] = ip
		}
		if len(aux) > 0 {
			cfg.AuxAddress = aux
		}
	}
	return cfg
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return r.Api(c, r.WithError(fmt.Errorf("name is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	driver := strings.TrimSpace(body.Driver)
	if driver == "" {
		driver = "bridge"
	}
	opts := network.CreateOptions{
		Driver:     driver,
		Options:    body.Options,
		Internal:   body.Internal,
		Attachable: body.Attachable,
		EnableIPv6: &body.EnableIPv6,
		Labels:     body.Labels,
	}

	var ipamCfgs []network.IPAMConfig
	if body.IPv4 != nil {
		if cfg := ipamFromBody(body.IPv4); cfg != nil {
			ipamCfgs = append(ipamCfgs, *cfg)
		}
	} else if subnet := strings.TrimSpace(body.Subnet); subnet != "" {
		cfg := network.IPAMConfig{Subnet: subnet}
		if g := strings.TrimSpace(body.Gateway); g != "" {
			cfg.Gateway = g
		}
		if ir := strings.TrimSpace(body.IPRange); ir != "" {
			cfg.IPRange = ir
		}
		ipamCfgs = append(ipamCfgs, cfg)
	}
	if body.IPv6 != nil {
		if cfg := ipamFromBody(body.IPv6); cfg != nil {
			ipamCfgs = append(ipamCfgs, *cfg)
		}
	}
	if len(ipamCfgs) > 0 {
		opts.IPAM = &network.IPAM{Config: ipamCfgs}
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	resp, err := cli.NetworkCreate(ctx, name, opts)
	if err != nil {
		return cc.respondErr(c, err)
	}
	payload, err := networkDetail(ctx, cli, resp.ID)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    fiber.Map{"id": resp.ID},
			"message": "Network created",
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    payload,
		"message": "Network created",
	}))
}

type networkContainer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	EndpointID  string `json:"endpoint_id,omitempty"`
	MacAddress  string `json:"mac_address,omitempty"`
	IPv4Address string `json:"ipv4_address,omitempty"`
	IPv6Address string `json:"ipv6_address,omitempty"`
}

func networkDetail(ctx context.Context, cli interface {
	NetworkInspect(context.Context, string, network.InspectOptions) (network.Inspect, error)
}, id string) (fiber.Map, error) {
	insp, err := cli.NetworkInspect(ctx, id, network.InspectOptions{Verbose: true})
	if err != nil {
		return nil, err
	}
	short := insp.ID
	if len(short) > 12 {
		short = short[:12]
	}
	containers := make([]networkContainer, 0, len(insp.Containers))
	for cid, ep := range insp.Containers {
		name := ep.Name
		if name == "" {
			name = cid
			if len(name) > 12 {
				name = name[:12]
			}
		}
		containers = append(containers, networkContainer{
			ID:          cid,
			Name:        name,
			EndpointID:  ep.EndpointID,
			MacAddress:  ep.MacAddress,
			IPv4Address: ep.IPv4Address,
			IPv6Address: ep.IPv6Address,
		})
	}

	var ipv4, ipv6 fiber.Map
	enableIPv6 := insp.EnableIPv6
	if insp.IPAM.Config != nil {
		for _, cfg := range insp.IPAM.Config {
			entry := fiber.Map{
				"subnet":   cfg.Subnet,
				"gateway":  cfg.Gateway,
				"ip_range": cfg.IPRange,
			}
			if len(cfg.AuxAddress) > 0 {
				excluded := make([]string, 0, len(cfg.AuxAddress))
				for _, v := range cfg.AuxAddress {
					excluded = append(excluded, v)
				}
				entry["excluded_ips"] = excluded
			}
			if strings.Contains(cfg.Subnet, ":") {
				ipv6 = entry
				enableIPv6 = true
			} else if cfg.Subnet != "" {
				ipv4 = entry
			}
		}
	}

	return fiber.Map{
		"id":          insp.ID,
		"short_id":    short,
		"name":        insp.Name,
		"driver":      insp.Driver,
		"scope":       insp.Scope,
		"internal":    insp.Internal,
		"attachable":  insp.Attachable,
		"ingress":     insp.Ingress,
		"enable_ipv6": enableIPv6,
		"options":     insp.Options,
		"labels":      insp.Labels,
		"created":     insp.Created.Format(time.RFC3339),
		"ipv4":        ipv4,
		"ipv6":        ipv6,
		"containers":  containers,
		"raw":         insp,
	}, nil
}

func (cc *controller) InspectAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	payload, err := networkDetail(ctx, cli, c.Params("id"))
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": payload}))
}

type disconnectBody struct {
	ContainerID string `json:"container_id"`
	Force       bool   `json:"force"`
}

func (cc *controller) DisconnectAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body disconnectBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	cid := strings.TrimSpace(body.ContainerID)
	if cid == "" {
		return r.Api(c, r.WithError(fmt.Errorf("container_id is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.NetworkDisconnect(ctx, c.Params("id"), cid, body.Force); err != nil {
		return cc.respondErr(c, err)
	}
	payload, _ := networkDetail(ctx, cli, c.Params("id"))
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    payload,
		"message": "Container disconnected",
	}))
}

func (cc *controller) RemoveAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.NetworkRemove(ctx, id); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Network removed",
	}))
}
