package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adyanth/cloudflare-operator/controllers"
	cf "github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
)

type credfile struct {
	AccountTag   string
	TunnelID     string
	TunnelName   string
	TunnelSecret string
}

type cfGoClient struct {
	accountID    string
	accountName  string
	tunnelName   string
	tunnelID     string
	tunnelSecret string
	domain       string
	records      map[string]cf.DNSRecord
	err          error

	cleanedUp bool
	deleted   bool
}

func (c *cfGoClient) CreateTunnel(ctx context.Context, rc *cf.ResourceContainer, params cf.TunnelCreateParams) (cf.Tunnel, error) {
	if params.Name != c.tunnelName {
		return cf.Tunnel{}, fmt.Errorf("Invalid tunnel name")
	}
	now := time.Now()
	c.tunnelSecret = params.Secret
	return cf.Tunnel{
		ID:           c.tunnelID,
		Name:         c.tunnelName,
		Secret:       params.Secret,
		CreatedAt:    &now,
		RemoteConfig: false,
	}, c.err
}

func (c *cfGoClient) CleanupTunnelConnections(ctx context.Context, rc *cf.ResourceContainer, tunnelID string) error {
	c.cleanedUp = true
	return c.err
}

func (c *cfGoClient) DeleteTunnel(ctx context.Context, rc *cf.ResourceContainer, tunnelID string) error {
	c.deleted = true
	return c.err
}

func (c *cfGoClient) Account(ctx context.Context, accountID string) (cf.Account, cf.ResultInfo, error) {
	return cf.Account{
		ID:        c.accountID,
		Name:      c.accountName,
		Type:      "",
		CreatedOn: time.Now(),
	}, cf.ResultInfo{}, c.err
}

func (c *cfGoClient) Accounts(ctx context.Context, params cf.AccountsListParams) ([]cf.Account, cf.ResultInfo, error) {
	if params.Name == c.accountName {
		return []cf.Account{{
			ID:        c.accountID,
			Name:      c.accountName,
			Type:      "",
			CreatedOn: time.Now(),
		}}, cf.ResultInfo{}, c.err
	}
	return []cf.Account{}, cf.ResultInfo{}, fmt.Errorf("No such account ID")
}

func (c *cfGoClient) GetTunnel(ctx context.Context, rc *cf.ResourceContainer, tunnelID string) (cf.Tunnel, error) {
	now := time.Now()
	return cf.Tunnel{
		ID:           c.tunnelID,
		Name:         c.tunnelName,
		Secret:       c.tunnelSecret,
		CreatedAt:    &now,
		RemoteConfig: false,
	}, c.err
}

func (c *cfGoClient) ListTunnels(ctx context.Context, rc *cf.ResourceContainer, params cf.TunnelListParams) ([]cf.Tunnel, *cf.ResultInfo, error) {
	if params.Name == c.tunnelName {
		now := time.Now()
		return []cf.Tunnel{{
			ID:           c.tunnelID,
			Name:         c.tunnelName,
			Secret:       c.tunnelSecret,
			CreatedAt:    &now,
			RemoteConfig: false,
		}}, &cf.ResultInfo{}, c.err
	}
	return []cf.Tunnel{}, &cf.ResultInfo{}, fmt.Errorf("No such tunnel")
}

func (c *cfGoClient) ListZones(ctx context.Context, z ...string) ([]cf.Zone, error) {
	if strings.HasSuffix(z[0], c.domain) {
		return []cf.Zone{{
			ID:   "zone-id",
			Name: "zone-name",
		}}, c.err
	}
	return []cf.Zone{}, fmt.Errorf("No such zone")
}

func (c *cfGoClient) UpdateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.UpdateDNSRecordParams) error {
	if rec, ok := c.records[params.ID]; ok && (params.Name == "" || params.Name == rec.Name) && params.Type == rec.Type {
		rec.Content = params.Content
		c.records[params.ID] = rec
		return c.err
	}
	return fmt.Errorf("Error updating DNS")
}

func (c *cfGoClient) CreateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.CreateDNSRecordParams) (*cf.DNSRecordResponse, error) {
	id := fmt.Sprint(len(c.records))
	c.records[id] = cf.DNSRecord{
		ID:      id,
		Type:    params.Type,
		Name:    params.Name,
		Content: params.Content,
	}
	return &cf.DNSRecordResponse{
		Result: c.records[id],
	}, c.err
}

func (c *cfGoClient) DeleteDNSRecord(ctx context.Context, rc *cf.ResourceContainer, recordID string) error {
	if _, ok := c.records[recordID]; ok {
		delete(c.records, recordID)
		return c.err
	}
	return fmt.Errorf("no record to delete")
}

func (c *cfGoClient) ListDNSRecords(ctx context.Context, rc *cf.ResourceContainer, params cf.ListDNSRecordsParams) ([]cf.DNSRecord, *cf.ResultInfo, error) {
	recs := []cf.DNSRecord{}
	for _, v := range c.records {
		if params.Type == v.Type && (params.Name == v.Name || params.Content == v.Comment) {
			recs = append(recs, v)
		}
	}
	return recs, &cf.ResultInfo{}, c.err
}

func setup(t *testing.T) (*controllers.CloudflareAPI, *cfGoClient) {
	cc := cfGoClient{
		accountName:  "account-name",
		accountID:    "account-id",
		tunnelName:   "tunnel-name",
		tunnelID:     "tunnel-id",
		tunnelSecret: "secret",
		domain:       "domain",
		err:          nil,
	}
	return &controllers.CloudflareAPI{
		Log:              testr.New(t),
		TunnelName:       cc.tunnelName,
		TunnelId:         cc.tunnelID,
		AccountName:      cc.accountName,
		AccountId:        cc.accountID,
		Domain:           cc.domain,
		APIToken:         "api-token",
		APIKey:           "api-key",
		APIEmail:         "api-email",
		CloudflareClient: &cc,
	}, &cc
}

func TestCreateCloudflareTunnel(t *testing.T) {
	c, cc := setup(t)
	id, creds, err := c.CreateCloudflareTunnel()
	if assert.NoError(t, err, "expected no error") {
		assert.Equal(t, c.TunnelId, id)
		v := credfile{}
		if assert.NoError(t, json.Unmarshal([]byte(creds), &v)) {
			assert.Equal(t, cc.accountID, v.AccountTag)
			assert.Equal(t, cc.tunnelID, v.TunnelID)
			assert.Equal(t, cc.tunnelName, v.TunnelName)
			assert.Equal(t, cc.tunnelSecret, v.TunnelSecret)
		}
	}
}

func TestDeleteCloudflareTunnel(t *testing.T) {
	c, cc := setup(t)
	err := c.DeleteCloudflareTunnel()
	if assert.NoError(t, err, "expected no error") {
		assert.Equal(t, true, cc.deleted)
	}
}
