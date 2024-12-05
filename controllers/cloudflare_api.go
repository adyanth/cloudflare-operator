package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
)

// TXT_PREFIX is the prefix added to TXT records for whom the corresponding DNS records are managed by the operator.
const TXT_PREFIX = "_managed."

// CloudflareAPI config object holding all relevant fields to use the API
type CloudflareAPI struct {
	Log              logr.Logger
	TunnelName       string
	TunnelId         string
	AccountName      string
	AccountId        string
	Domain           string
	APIToken         string
	APIKey           string
	APIEmail         string
	ValidAccountId   string
	ValidTunnelId    string
	ValidTunnelName  string
	ValidZoneId      string
	CloudflareClient *cloudflare.API
}

// CloudflareTunnelCredentialsFile object containing the fields that make up a Cloudflare Tunnel's credentials
type CloudflareTunnelCredentialsFile struct {
	AccountTag   string `json:"AccountTag"`
	TunnelID     string `json:"TunnelID"`
	TunnelName   string `json:"TunnelName"`
	TunnelSecret string `json:"TunnelSecret"`
}

// DnsManagedRecordTxt object that represents each managed DNS record in a separate TXT record
type DnsManagedRecordTxt struct {
	DnsId      string // DnsId of the managed record
	TunnelName string // TunnelName of the managed record
	TunnelId   string // TunnelId of the managed record
}

// CreateCloudflareTunnel creates a Cloudflare Tunnel and returns the tunnel Id and credentials file
func (c *CloudflareAPI) CreateCloudflareTunnel() (string, string, error) {
	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error code in getting account ID")
		return "", "", err
	}

	// Generate 32 byte random string for tunnel secret
	randSecret := make([]byte, 32)
	if _, err := rand.Read(randSecret); err != nil {
		return "", "", err
	}
	tunnelSecret := base64.StdEncoding.EncodeToString(randSecret)

	params := cloudflare.TunnelCreateParams{
		Name:   c.TunnelName,
		Secret: tunnelSecret,
		// Indicates if this is a locally or remotely configured tunnel "local" or "cloudflare"
		ConfigSrc: "local",
	}

	ctx := context.Background()
	rc := cloudflare.AccountIdentifier(c.ValidAccountId)
	tunnel, err := c.CloudflareClient.CreateTunnel(ctx, rc, params)

	if err != nil {
		c.Log.Error(err, "error creating tunnel")
		return "", "", err
	}

	c.ValidTunnelId = tunnel.ID
	c.ValidTunnelName = tunnel.Name

	credentialsFile := CloudflareTunnelCredentialsFile{
		AccountTag:   c.ValidAccountId,
		TunnelID:     tunnel.ID,
		TunnelName:   tunnel.Name,
		TunnelSecret: tunnelSecret,
	}

	// Marshal the tunnel credentials into a string
	creds, err := json.Marshal(credentialsFile)
	return tunnel.ID, string(creds), err
}

// DeleteCloudflareTunnel deletes a Cloudflare Tunnel
func (c *CloudflareAPI) DeleteCloudflareTunnel() error {
	if err := c.ValidateAll(); err != nil {
		c.Log.Error(err, "Error in validation")
		return err
	}

	ctx := context.Background()
	rc := cloudflare.AccountIdentifier(c.ValidAccountId)

	// Deletes any inactive connections on a tunnel
	err := c.CloudflareClient.CleanupTunnelConnections(ctx, rc, c.ValidTunnelId)
	if err != nil {
		c.Log.Error(err, "error cleaning tunnel connections", "tunnelId", c.TunnelId)
		return err
	}

	ctx = context.Background()
	err = c.CloudflareClient.DeleteTunnel(ctx, rc, c.ValidTunnelId)
	if err != nil {
		c.Log.Error(err, "error deleting tunnel", "tunnelId", c.TunnelId)
		return err
	}

	return nil
}

// ValidateAll validates the contents of the CloudflareAPI struct
func (c *CloudflareAPI) ValidateAll() error {
	c.Log.Info("In validation")
	if _, err := c.GetAccountId(); err != nil {
		return err
	}

	if _, err := c.GetTunnelId(); err != nil {
		return err
	}

	if _, err := c.GetZoneId(); err != nil {
		return err
	}

	c.Log.Info("Validation successful")
	return nil
}

// GetAccountId gets AccountId from Account Name
func (c *CloudflareAPI) GetAccountId() (string, error) {
	if c.ValidAccountId != "" {
		return c.ValidAccountId, nil
	}

	if c.AccountId == "" && c.AccountName == "" {
		err := fmt.Errorf("both account ID and Name cannot be empty")
		c.Log.Error(err, "Both accountId and accountName cannot be empty")
		return "", err
	}

	if c.validateAccountId() {
		c.ValidAccountId = c.AccountId
	} else {
		c.Log.Info("Account ID failed, falling back to Account Name")
		accountIdFromName, err := c.getAccountIdByName()
		if err != nil {
			return "", fmt.Errorf("error fetching Account ID by Account Name")
		}
		c.ValidAccountId = accountIdFromName
	}
	return c.ValidAccountId, nil
}

func (c CloudflareAPI) validateAccountId() bool {
	if c.AccountId == "" {
		c.Log.Info("Account ID not provided")
		return false
	}

	ctx := context.Background()
	account, _, err := c.CloudflareClient.Account(ctx, c.AccountId)

	if err != nil {
		c.Log.Error(err, "error retrieving account details", "accountId", c.AccountId)
		return false
	}

	return account.ID == c.AccountId
}

func (c *CloudflareAPI) getAccountIdByName() (string, error) {
	ctx := context.Background()
	params := cloudflare.AccountsListParams{
		Name: c.AccountName,
	}
	accounts, _, err := c.CloudflareClient.Accounts(ctx, params)

	if err != nil {
		c.Log.Error(err, "error listing accounts", "accountName", c.AccountName)
	}

	switch len(accounts) {
	case 0:
		err := fmt.Errorf("no account in response")
		c.Log.Error(err, "found no account, check accountName", "accountName", c.AccountName)
		return "", err
	case 1:
		return accounts[0].ID, nil
	default:
		err := fmt.Errorf("more than one account in response")
		c.Log.Error(err, "found more than one account, check accountName", "accountName", c.AccountName)
		return "", err
	}
}

// GetTunnelId gets Tunnel Id from available information
func (c *CloudflareAPI) GetTunnelId() (string, error) {
	if c.ValidTunnelId != "" {
		return c.ValidTunnelId, nil
	}

	if c.TunnelId == "" && c.TunnelName == "" {
		err := fmt.Errorf("both tunnel ID and Name cannot be empty")
		c.Log.Error(err, "Both tunnelId and tunnelName cannot be empty")
		return "", err
	}

	if c.validateTunnelId() {
		c.ValidTunnelId = c.TunnelId
		return c.TunnelId, nil
	}

	c.Log.Info("Tunnel ID failed, falling back to Tunnel Name")
	tunnelIdFromName, err := c.getTunnelIdByName()
	if err != nil {
		return "", fmt.Errorf("error fetching Tunnel ID by Tunnel Name")
	}
	c.ValidTunnelId = tunnelIdFromName
	c.ValidTunnelName = c.TunnelName

	return c.ValidTunnelId, nil
}

func (c *CloudflareAPI) validateTunnelId() bool {
	if c.TunnelId == "" {
		c.Log.Info("Tunnel ID not provided")
		return false
	}

	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error in getting account ID")
		return false
	}

	ctx := context.Background()
	rc := cloudflare.AccountIdentifier(c.ValidAccountId)
	tunnel, err := c.CloudflareClient.GetTunnel(ctx, rc, c.TunnelId)
	if err != nil {
		c.Log.Error(err, "error retrieving tunnel", "tunnelId", c.TunnelId)
		return false
	}

	c.ValidTunnelName = tunnel.Name
	return tunnel.ID == c.TunnelId
}

func (c *CloudflareAPI) getTunnelIdByName() (string, error) {
	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error in getting account ID")
		return "", err
	}

	ctx := context.Background()
	rc := cloudflare.AccountIdentifier(c.ValidAccountId)
	params := cloudflare.TunnelListParams{
		Name: c.TunnelName,
	}
	tunnels, _, err := c.CloudflareClient.ListTunnels(ctx, rc, params)

	if err != nil {
		c.Log.Error(err, "error listing tunnels by name", "tunnelName", c.TunnelName)
		return "", err
	}

	switch len(tunnels) {
	case 0:
		err := fmt.Errorf("no tunnel in response")
		c.Log.Error(err, "found no tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	case 1:
		c.ValidTunnelName = tunnels[0].Name
		return tunnels[0].ID, nil
	default:
		err := fmt.Errorf("more than one tunnel in response")
		c.Log.Error(err, "found more than one tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	}
}

// GetTunnelCreds gets Tunnel Credentials from Tunnel secret
func (c *CloudflareAPI) GetTunnelCreds(tunnelSecret string) (string, error) {
	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error in getting account ID")
		return "", err
	}

	if _, err := c.GetTunnelId(); err != nil {
		c.Log.Error(err, "error in getting tunnel ID")
		return "", err
	}

	creds, err := json.Marshal(map[string]string{
		"AccountTag":   c.ValidAccountId,
		"TunnelSecret": tunnelSecret,
		"TunnelID":     c.ValidTunnelId,
		"TunnelName":   c.ValidTunnelName,
	})

	return string(creds), err
}

// GetZoneId gets Zone Id from DNS domain
func (c *CloudflareAPI) GetZoneId() (string, error) {
	if c.ValidZoneId != "" {
		return c.ValidZoneId, nil
	}

	if c.Domain == "" {
		err := fmt.Errorf("domain cannot be empty")
		c.Log.Error(err, "Domain cannot be empty")
		return "", err
	}

	zoneIdFromName, err := c.getZoneIdByName()
	if err != nil {
		return "", fmt.Errorf("error fetching Zone ID by Zone Name")
	}
	c.ValidZoneId = zoneIdFromName
	return c.ValidZoneId, nil
}

func (c *CloudflareAPI) getZoneIdByName() (string, error) {
	ctx := context.Background()
	zones, err := c.CloudflareClient.ListZones(ctx, c.Domain)

	if err != nil {
		c.Log.Error(err, "error listing zones, check domain", "domain", c.Domain)
		return "", err
	}

	switch len(zones) {
	case 0:
		err := fmt.Errorf("no zone in response")
		c.Log.Error(err, "found no zone, check domain", "domain", c.Domain, "zones", zones)
		return "", err
	case 1:
		return zones[0].ID, nil
	default:
		err := fmt.Errorf("more than one zone in response")
		c.Log.Error(err, "found more than one zone, check domain", "domain", c.Domain)
		return "", err
	}
}

// Been a while writing Go... returns a pointer to a type
func ptr[T any](v T) *T {
	return &v
}

// InsertOrUpdateCName upsert DNS CNAME record for the given FQDN to point to the tunnel
func (c *CloudflareAPI) InsertOrUpdateCName(fqdn, dnsId string) (string, error) {
	ctx := context.Background()
	rc := cloudflare.ZoneIdentifier(c.ValidZoneId)
	if dnsId != "" {
		c.Log.Info("Updating existing record", "fqdn", fqdn, "dnsId", dnsId)
		updateParams := cloudflare.UpdateDNSRecordParams{
			ID:      dnsId,
			Type:    "CNAME",
			Name:    fqdn,
			Content: fmt.Sprintf("%s.cfargotunnel.com", c.ValidTunnelId),
			Comment: "Managed by cloudflare-operator",
			TTL:     1,         // Automatic TTL
			Proxied: ptr(true), // For Cloudflare tunnels
		}
		err := c.CloudflareClient.UpdateDNSRecord(ctx, rc, updateParams)
		if err != nil {
			c.Log.Error(err, "error code in setting/updating DNS record, check fqdn", "fqdn", fqdn)
			return "", err
		}
		c.Log.Info("DNS record updated successfully", "fqdn", fqdn)
		return dnsId, nil
	} else {
		c.Log.Info("Inserting DNS record", "fqdn", fqdn)
		createParams := cloudflare.CreateDNSRecordParams{
			Type:    "CNAME",
			Name:    fqdn,
			Content: fmt.Sprintf("%s.cfargotunnel.com", c.ValidTunnelId),
			Comment: "Managed by cloudflare-operator",
			TTL:     1,         // Automatic TTL
			Proxied: ptr(true), // For Cloudflare tunnels
		}
		resp, err := c.CloudflareClient.CreateDNSRecord(ctx, rc, createParams)
		if err != nil {
			c.Log.Error(err, "error creating DNS record, check fqdn", "fqdn", fqdn)
			return "", err
		}
		c.Log.Info("DNS record created successfully", "fqdn", fqdn)
		return resp.Result.ID, nil
	}
}

// DeleteDNSId deletes DNS entry for the given dnsId
func (c *CloudflareAPI) DeleteDNSId(fqdn, dnsId string, created bool) error {
	// Do not delete if we did not create the DNS in this cycle
	if !created {
		return nil
	}

	ctx := context.Background()
	rc := cloudflare.ZoneIdentifier(c.ValidZoneId)
	err := c.CloudflareClient.DeleteDNSRecord(ctx, rc, dnsId)

	if err != nil {
		c.Log.Error(err, "error deleting DNS record, check fqdn", "dnsId", dnsId, "fqdn", fqdn)
		return err
	}

	return nil
}

// GetDNSCNameId returns the ID of the CNAME record requested
func (c *CloudflareAPI) GetDNSCNameId(fqdn string) (string, error) {
	if _, err := c.GetZoneId(); err != nil {
		c.Log.Error(err, "error in getting Zone ID")
		return "", err
	}

	ctx := context.Background()
	rc := cloudflare.ZoneIdentifier(c.ValidZoneId)
	params := cloudflare.ListDNSRecordsParams{
		Type: "CNAME",
		Name: fqdn,
	}
	records, _, err := c.CloudflareClient.ListDNSRecords(ctx, rc, params)
	if err != nil {
		c.Log.Error(err, "error listing DNS records, check fqdn", "fqdn", fqdn)
		return "", err
	}

	switch len(records) {
	case 0:
		err := fmt.Errorf("no records returned")
		c.Log.Info("no records returned for fqdn", "fqdn", fqdn)
		return "", err
	case 1:
		return records[0].ID, nil
	default:
		err := fmt.Errorf("multiple records returned")
		c.Log.Error(err, "multiple records returned for fqdn", "fqdn", fqdn)
		return "", err
	}
}

// GetManagedDnsTxt gets the TXT record corresponding to the fqdn
func (c *CloudflareAPI) GetManagedDnsTxt(fqdn string) (string, DnsManagedRecordTxt, bool, error) {
	if _, err := c.GetZoneId(); err != nil {
		c.Log.Error(err, "error in getting Zone ID")
		return "", DnsManagedRecordTxt{}, false, err
	}

	ctx := context.Background()
	rc := cloudflare.ZoneIdentifier(c.ValidZoneId)
	params := cloudflare.ListDNSRecordsParams{
		Type: "TXT",
		Name: fmt.Sprintf("%s%s", TXT_PREFIX, fqdn),
	}
	records, _, err := c.CloudflareClient.ListDNSRecords(ctx, rc, params)
	if err != nil {
		c.Log.Error(err, "error listing DNS records, check fqdn", "fqdn", fqdn)
		return "", DnsManagedRecordTxt{}, false, err
	}

	switch len(records) {
	case 0:
		c.Log.Info("no TXT records returned for fqdn", "fqdn", fqdn)
		return "", DnsManagedRecordTxt{}, true, nil
	case 1:
		var dnsTxtResponse DnsManagedRecordTxt
		if err := json.Unmarshal([]byte(records[0].Content), &dnsTxtResponse); err != nil {
			// TXT record exists, but not in JSON
			c.Log.Error(err, "could not read TXT content in getting zoneId, check domain", "domain", c.Domain)
			return records[0].ID, dnsTxtResponse, false, err
		} else if dnsTxtResponse.TunnelId == c.ValidTunnelId {
			// TXT record exists and controlled by our tunnel
			return records[0].ID, dnsTxtResponse, true, nil
		}
	default:
		err := fmt.Errorf("multiple records returned")
		c.Log.Error(err, "multiple TXT records returned for fqdn", "fqdn", fqdn)
		return "", DnsManagedRecordTxt{}, false, err
	}
	return "", DnsManagedRecordTxt{}, false, err
}

// InsertOrUpdateTXT upsert DNS TXT record for the given FQDN to point to the tunnel
func (c *CloudflareAPI) InsertOrUpdateTXT(fqdn, txtId, dnsId string) error {
	content, err := json.Marshal(DnsManagedRecordTxt{
		DnsId:      dnsId,
		TunnelId:   c.ValidTunnelId,
		TunnelName: c.ValidTunnelName,
	})
	if err != nil {
		c.Log.Error(err, "error marhsalling txt record json", "fqdn", fqdn)
		return err
	}
	ctx := context.Background()
	rc := cloudflare.ZoneIdentifier(c.ValidZoneId)

	if txtId != "" {
		c.Log.Info("Updating existing TXT record", "fqdn", fqdn, "dnsId", dnsId, "txtId", txtId)

		updateParams := cloudflare.UpdateDNSRecordParams{
			ID:      txtId,
			Type:    "TXT",
			Name:    fmt.Sprintf("%s%s", TXT_PREFIX, fqdn),
			Content: string(content),
			Comment: "Managed by cloudflare-operator",
			TTL:     1,          // Automatic TTL
			Proxied: ptr(false), // TXT cannot be proxied
		}
		err := c.CloudflareClient.UpdateDNSRecord(ctx, rc, updateParams)
		if err != nil {
			c.Log.Error(err, "error in updating DNS record, check fqdn", "fqdn", fqdn)
			return err
		}
		c.Log.Info("DNS record updated successfully", "fqdn", fqdn)
		return nil
	} else {
		c.Log.Info("Inserting DNS TXT record", "fqdn", fqdn)
		createParams := cloudflare.CreateDNSRecordParams{
			Type:    "TXT",
			Name:    fmt.Sprintf("%s%s", TXT_PREFIX, fqdn),
			Content: string(content),
			Comment: "Managed by cloudflare-operator",
			TTL:     1,          // Automatic TTL
			Proxied: ptr(false), // For Cloudflare tunnels
		}
		_, err := c.CloudflareClient.CreateDNSRecord(ctx, rc, createParams)
		if err != nil {
			c.Log.Error(err, "error creating DNS record, check fqdn", "fqdn", fqdn)
			return err
		}
		c.Log.Info("DNS TXT record created successfully", "fqdn", fqdn)
		return nil
	}
}

func (c *CloudflareAPI) getAccessApplicationIdByName(name string) (exists bool, id string, error error) {
	ctx := context.Background()
	accountId := c.ValidAccountId

	apps, _, err := c.CloudflareClient.AccessApplications(ctx, accountId, cloudflare.PaginationOptions{})
	if err != nil {
		return false, "", err
	}

	exists = false
	for _, app := range apps {
		if app.Name == name {
			exists = true
			id = app.ID
			break
		}
	}
	return exists, id, nil
}

func (c *CloudflareAPI) CreateAccessConfig(name string, config networkingv1alpha1.AccessConfig) error {
	ctx := context.Background()
	newApp := config.NewAccessApplication(name)
	accountId := c.ValidAccountId

	exists, id, err := c.getAccessApplicationIdByName(name)
	if err != nil {
		c.Log.Error(err, "failed retrieving application for account", "accountId", accountId, "app", name)
	}

	if exists {
		newApp.ID = id
		c.Log.Info("updating access application", "name", name, "id", id)
		_, err := c.CloudflareClient.UpdateAccessApplication(ctx, accountId, newApp)
		if err != nil {
			c.Log.Error(err, "error updating access application", "name", name)
			return err
		}
	} else {
		c.Log.Info("creating access application", "name", name)
		newApp.ID = ""
		_, err := c.CloudflareClient.CreateAccessApplication(ctx, accountId, newApp)
		if err != nil {
			c.Log.Error(err, "error creating access application", "name", name)
			return err
		}
	}

	c.Log.Info("access application reconciled successfully", "name", name, "existing", exists)
	return nil
}

func (c *CloudflareAPI) DeleteAccessConfig(name string, config networkingv1alpha1.AccessConfig) error {
	ctx := context.Background()
	accountId := c.ValidAccountId

	exists, id, err := c.getAccessApplicationIdByName(name)
	if err != nil {
		c.Log.Error(err, "failed retrieving application for account", "accountId", accountId, "app", name)
	}

	if exists {
		c.Log.Info("deleting access application", "name", name, "id", id)
		err := c.CloudflareClient.DeleteAccessApplication(ctx, accountId, id)
		if err != nil {
			c.Log.Error(err, "error deleting access application", "name", name)
			return err
		}
	} else {
		err := fmt.Errorf("application does not exist", "name", name, "id", id)
		return err
	}

	c.Log.Info("access application deleted successfully", "name", name, "existing", exists)
	return nil
}
