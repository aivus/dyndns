package updater

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	cloudflare "github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/dns"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/cloudflare/cloudflare-go/v6/zones"

	"github.com/aivus/dyndns/internal/config"
)

// Record is a minimal representation of a Cloudflare DNS record.
type Record struct {
	ID      string
	Content string
}

// DNSClient abstracts Cloudflare DNS operations, enabling test mocks.
type DNSClient interface {
	ListRecords(ctx context.Context, zoneID, name string) ([]Record, error)
	UpdateRecord(ctx context.Context, zoneID, recordID, name, ip string) error
	CreateRecord(ctx context.Context, zoneID, name, ip string) error
}

// Updater applies IPv6 prefix updates to all configured DNS records.
type Updater struct {
	client  DNSClient
	records []config.RecordConfig
}

func New(client DNSClient, records []config.RecordConfig) *Updater {
	return &Updater{client: client, records: records}
}

// Update computes a new AAAA address for each configured record and upserts it
// via Cloudflare. Records with a suffix combine prefix+suffix; records without a
// suffix use routerIP directly. It is idempotent when the IP is unchanged.
func (u *Updater) Update(ctx context.Context, prefix, routerIP string) error {
	for _, rec := range u.records {
		var ip string
		if rec.Suffix != "" {
			var err error
			ip, err = CombinePrefix(prefix, rec.Suffix)
			if err != nil {
				return fmt.Errorf("record %s: %w", rec.Name, err)
			}
		} else {
			if routerIP == "" {
				return fmt.Errorf("record %q has no suffix but ip6addr was not provided", rec.Name)
			}
			ip = routerIP
		}
		if err := u.upsert(ctx, rec, ip); err != nil {
			return fmt.Errorf("record %s: %w", rec.Name, err)
		}
	}
	return nil
}

func (u *Updater) upsert(ctx context.Context, rec config.RecordConfig, ip string) error {
	existing, err := u.client.ListRecords(ctx, rec.ZoneID, rec.Name)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		slog.Info("creating AAAA record", "name", rec.Name, "ip", ip)
		return u.client.CreateRecord(ctx, rec.ZoneID, rec.Name, ip)
	}
	for _, r := range existing {
		if r.Content == ip {
			slog.Debug("AAAA record already up to date", "name", rec.Name, "ip", ip)
			return nil
		}
	}
	slog.Info("updating AAAA record", "name", rec.Name, "id", existing[0].ID, "ip", ip)
	return u.client.UpdateRecord(ctx, rec.ZoneID, existing[0].ID, rec.Name, ip)
}

// CombinePrefix combines an IPv6 CIDR prefix (e.g. "2001:db8::/64") with a
// static host suffix (e.g. "::1") to produce a full IPv6 address string.
func CombinePrefix(prefix, suffix string) (string, error) {
	_, ipNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return "", fmt.Errorf("invalid prefix %q: %w", prefix, err)
	}

	suffixIP := net.ParseIP(suffix)
	if suffixIP == nil {
		return "", fmt.Errorf("invalid suffix %q", suffix)
	}
	suffixIP = suffixIP.To16()

	network := ipNet.IP.To16()
	if network == nil {
		return "", fmt.Errorf("prefix %q has no valid IPv6 network address", prefix)
	}

	result := make(net.IP, 16)
	for i := 0; i < 16; i++ {
		result[i] = network[i] | suffixIP[i]
	}

	return result.String(), nil
}

// cloudflareClient wraps the real Cloudflare SDK to implement DNSClient.
type cloudflareClient struct {
	cl         *cloudflare.Client
	zoneNames  sync.Map // zoneID → zone domain name (string)
}

// NewCloudflareClient creates a production DNSClient backed by the Cloudflare API.
func NewCloudflareClient(apiToken string) DNSClient {
	return &cloudflareClient{
		cl: cloudflare.NewClient(option.WithAPIToken(apiToken)),
	}
}

// zoneName returns the domain name for a zone ID, caching results so the
// extra API call only happens once per zone per process lifetime.
func (c *cloudflareClient) zoneName(ctx context.Context, zoneID string) (string, error) {
	if v, ok := c.zoneNames.Load(zoneID); ok {
		return v.(string), nil
	}
	zone, err := c.cl.Zones.Get(ctx, zones.ZoneGetParams{ZoneID: cloudflare.F(zoneID)})
	if err != nil {
		return "", fmt.Errorf("lookup zone %s: %w", zoneID, err)
	}
	c.zoneNames.Store(zoneID, zone.Name)
	return zone.Name, nil
}

func (c *cloudflareClient) ListRecords(ctx context.Context, zoneID, name string) ([]Record, error) {
	zoneName, err := c.zoneName(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	fqdn := toFQDN(name, zoneName)
	pager := c.cl.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name:   cloudflare.F(dns.RecordListParamsName{Exact: cloudflare.F(fqdn)}),
		Type:   cloudflare.F(dns.RecordListParamsTypeAAAA),
	})
	var out []Record
	for pager.Next() {
		r := pager.Current()
		out = append(out, Record{ID: r.ID, Content: r.Content})
	}
	if err := pager.Err(); err != nil {
		return nil, err
	}
	slog.Debug("cloudflare AAAA record list", "zone_id", zoneID, "fqdn", fqdn, "count", len(out))
	return out, nil
}

// toFQDN returns the absolute DNS name for a record. If name is already a
// subdomain of zoneName (or equals it), it is returned unchanged; otherwise
// zoneName is appended. Comparison is case-insensitive.
func toFQDN(name, zoneName string) string {
	n := strings.ToLower(strings.TrimSuffix(name, "."))
	z := strings.ToLower(strings.TrimSuffix(zoneName, "."))
	if n == z || strings.HasSuffix(n, "."+z) {
		return n
	}
	return n + "." + z
}

func (c *cloudflareClient) UpdateRecord(ctx context.Context, zoneID, recordID, name, ip string) error {
	_, err := c.cl.DNS.Records.Update(ctx, recordID, dns.RecordUpdateParams{
		ZoneID: cloudflare.F(zoneID),
		Body: dns.AAAARecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Content: cloudflare.F(ip),
			TTL:     cloudflare.F(dns.TTL(60)),
		},
	})
	return err
}

func (c *cloudflareClient) CreateRecord(ctx context.Context, zoneID, name, ip string) error {
	_, err := c.cl.DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Body: dns.AAAARecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Content: cloudflare.F(ip),
			TTL:     cloudflare.F(dns.TTL(60)),
		},
	})
	return err
}
