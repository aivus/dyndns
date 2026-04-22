package updater

import (
	"context"
	"fmt"
	"net"

	cloudflare "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"

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

// Update computes a new AAAA address for each configured record from the given
// prefix and upserts it via Cloudflare. It is idempotent when the IP is unchanged.
func (u *Updater) Update(ctx context.Context, prefix string) error {
	for _, rec := range u.records {
		ip, err := CombinePrefix(prefix, rec.Suffix)
		if err != nil {
			return fmt.Errorf("record %s: %w", rec.Name, err)
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
		return u.client.CreateRecord(ctx, rec.ZoneID, rec.Name, ip)
	}
	for _, r := range existing {
		if r.Content == ip {
			continue
		}
		if err := u.client.UpdateRecord(ctx, rec.ZoneID, r.ID, rec.Name, ip); err != nil {
			return err
		}
	}
	return nil
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
	cl *cloudflare.Client
}

// NewCloudflareClient creates a production DNSClient backed by the Cloudflare API.
func NewCloudflareClient(apiToken string) DNSClient {
	return &cloudflareClient{
		cl: cloudflare.NewClient(option.WithAPIToken(apiToken)),
	}
}

func (c *cloudflareClient) ListRecords(ctx context.Context, zoneID, name string) ([]Record, error) {
	resp, err := c.cl.DNS.Records.List(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name:   cloudflare.F(dns.RecordListParamsName{Exact: cloudflare.F(name)}),
		Type:   cloudflare.F(dns.RecordListParamsTypeAAAA),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(resp.Result))
	for _, r := range resp.Result {
		out = append(out, Record{ID: r.ID, Content: r.Content})
	}
	return out, nil
}

func (c *cloudflareClient) UpdateRecord(ctx context.Context, zoneID, recordID, name, ip string) error {
	_, err := c.cl.DNS.Records.Update(ctx, recordID, dns.RecordUpdateParams{
		ZoneID: cloudflare.F(zoneID),
		Body: dns.AAAARecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Content: cloudflare.F(ip),
			TTL:     cloudflare.F(dns.TTL(1)),
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
			TTL:     cloudflare.F(dns.TTL(1)),
		},
	})
	return err
}
