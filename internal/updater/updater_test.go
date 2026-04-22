package updater

import (
	"context"
	"errors"
	"testing"

	"github.com/aivus/dyndns/internal/config"
)

// TestCombinePrefix covers the prefix+suffix merging logic.
func TestCombinePrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		suffix  string
		want    string
		wantErr bool
	}{
		{
			name:   "::1 on /64",
			prefix: "2001:db8:1234:5601::/64",
			suffix: "::1",
			want:   "2001:db8:1234:5601::1",
		},
		{
			name:   "::cafe:1 on /64",
			prefix: "2001:db8:1234:5601::/64",
			suffix: "::cafe:1",
			want:   "2001:db8:1234:5601::cafe:1",
		},
		{
			name:   "::1 on /48",
			prefix: "fd00::/48",
			suffix: "::1",
			want:   "fd00::1",
		},
		{
			name:    "invalid prefix",
			prefix:  "notaprefix",
			suffix:  "::1",
			wantErr: true,
		},
		{
			name:    "invalid suffix",
			prefix:  "2001:db8::/64",
			suffix:  "notasuffix",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := combinePrefix(tc.prefix, tc.suffix)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("CombinePrefix(%q, %q) = %q, want %q", tc.prefix, tc.suffix, got, tc.want)
			}
		})
	}
}

// mockDNSClient captures calls for assertion in tests.
type mockDNSClient struct {
	listFn   func(ctx context.Context, zoneID, name string) ([]Record, error)
	updateFn func(ctx context.Context, zoneID, recordID, name, ip string) error
	createFn func(ctx context.Context, zoneID, name, ip string) error

	updateCalled bool
	createCalled bool
}

func (m *mockDNSClient) ListRecords(ctx context.Context, zoneID, name string) ([]Record, error) {
	return m.listFn(ctx, zoneID, name)
}

func (m *mockDNSClient) UpdateRecord(ctx context.Context, zoneID, recordID, name, ip string) error {
	m.updateCalled = true
	return m.updateFn(ctx, zoneID, recordID, name, ip)
}

func (m *mockDNSClient) CreateRecord(ctx context.Context, zoneID, name, ip string) error {
	m.createCalled = true
	return m.createFn(ctx, zoneID, name, ip)
}

var testRecords = []config.RecordConfig{
	{ZoneID: "zone1", Name: "home.example.com", Suffix: "::1"},
}

func TestUpdater_RecordExistsWithDifferentIP(t *testing.T) {
	var updatedIP string
	mock := &mockDNSClient{
		listFn: func(_ context.Context, _, _ string) ([]Record, error) {
			return []Record{{ID: "rec1", Content: "2001:db8::ff"}}, nil
		},
		updateFn: func(_ context.Context, _, _, _, ip string) error {
			updatedIP = ip
			return nil
		},
	}
	u := New(mock, testRecords)
	if err := u.Update(context.Background(), "2001:db8:1234:5601::/64", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.updateCalled {
		t.Error("expected UpdateRecord to be called")
	}
	if mock.createCalled {
		t.Error("expected CreateRecord NOT to be called")
	}
	if updatedIP != "2001:db8:1234:5601::1" {
		t.Errorf("updated IP = %q, want %q", updatedIP, "2001:db8:1234:5601::1")
	}
}

func TestUpdater_RecordExistsWithSameIP(t *testing.T) {
	mock := &mockDNSClient{
		listFn: func(_ context.Context, _, _ string) ([]Record, error) {
			return []Record{{ID: "rec1", Content: "2001:db8:1234:5601::1"}}, nil
		},
	}
	u := New(mock, testRecords)
	if err := u.Update(context.Background(), "2001:db8:1234:5601::/64", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.updateCalled {
		t.Error("expected UpdateRecord NOT to be called (idempotent)")
	}
	if mock.createCalled {
		t.Error("expected CreateRecord NOT to be called (idempotent)")
	}
}

func TestUpdater_RecordDoesNotExist(t *testing.T) {
	var createdIP string
	mock := &mockDNSClient{
		listFn: func(_ context.Context, _, _ string) ([]Record, error) {
			return nil, nil
		},
		createFn: func(_ context.Context, _, _, ip string) error {
			createdIP = ip
			return nil
		},
	}
	u := New(mock, testRecords)
	if err := u.Update(context.Background(), "2001:db8:1234:5601::/64", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.createCalled {
		t.Error("expected CreateRecord to be called")
	}
	if mock.updateCalled {
		t.Error("expected UpdateRecord NOT to be called")
	}
	if createdIP != "2001:db8:1234:5601::1" {
		t.Errorf("created IP = %q, want %q", createdIP, "2001:db8:1234:5601::1")
	}
}

func TestToFQDN(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		want     string
	}{
		{"home", "example.com", "home.example.com"},
		{"home.example.com", "example.com", "home.example.com"},   // already FQDN
		{"HOME", "Example.COM", "home.example.com"},               // case-insensitive
		{"home.base", "example.com", "home.base.example.com"},     // multi-label config
		{"home.base.example.com", "example.com", "home.base.example.com"}, // already FQDN multi-label
		{"example.com", "example.com", "example.com"},             // apex
	}
	for _, tc := range tests {
		got := toFQDN(tc.name, tc.zone)
		if got != tc.want {
			t.Errorf("toFQDN(%q, %q) = %q, want %q", tc.name, tc.zone, got, tc.want)
		}
	}
}

func TestUpdater_CloudflareAPIError(t *testing.T) {
	apiErr := errors.New("cloudflare: rate limit exceeded")
	mock := &mockDNSClient{
		listFn: func(_ context.Context, _, _ string) ([]Record, error) {
			return nil, apiErr
		},
	}
	u := New(mock, testRecords)
	err := u.Update(context.Background(), "2001:db8:1234:5601::/64", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error = %v, want to wrap %v", err, apiErr)
	}
}

var testRecordsNoSuffix = []config.RecordConfig{
	{ZoneID: "zone1", Name: "router.example.com"},
}

func TestUpdater_RouterIPUsedWhenNoSuffix(t *testing.T) {
	const routerIP = "2001:db8::abcd"
	var createdIP string
	mock := &mockDNSClient{
		listFn: func(_ context.Context, _, _ string) ([]Record, error) {
			return nil, nil
		},
		createFn: func(_ context.Context, _, _, ip string) error {
			createdIP = ip
			return nil
		},
	}
	u := New(mock, testRecordsNoSuffix)
	if err := u.Update(context.Background(), "", routerIP); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.createCalled {
		t.Error("expected CreateRecord to be called")
	}
	if createdIP != routerIP {
		t.Errorf("created IP = %q, want %q", createdIP, routerIP)
	}
}

func TestUpdater_MissingRouterIPForNoSuffixRecord(t *testing.T) {
	u := New(&mockDNSClient{}, testRecordsNoSuffix)
	err := u.Update(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
