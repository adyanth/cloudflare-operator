package controllers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

const CLOUDFLARE_ENDPOINT = "https://api.cloudflare.com/client/v4/"

type CloudflarAPI struct {
	Log             logr.Logger
	TunnelName      string
	TunnelId        string
	AccountName     string
	AccountId       string
	Domain          string
	APIToken        string
	APIKey          string
	APIEmail        string
	ValidAccountId  string
	ValidTunnelId   string
	ValidTunnelName string
	ValidZoneId     string
}

type CloudflareAPINameResponse struct {
	Result []struct {
		Id   string
		Name string
	}
	Errors []struct {
		Message string
	}
	Success bool
}
type CloudflareAPIIdResponse struct {
	Result struct {
		Id   string
		Name string
	}
	Errors []struct {
		Message string
	}
	Success bool
}

type CloudflareAPITunnelResponse struct {
	Result struct {
		Id              string
		Name            string
		CredentialsFile map[string]string `json:"credentials_file"`
	}
	Success bool
	Errors  []struct {
		Message string
	}
}

type CloudflareAPITunnelCreate struct {
	Name         string
	TunnelSecret string `json:"tunnel_secret"`
}

func (c CloudflarAPI) addAuthHeader(req *http.Request, delete bool) error {
	if !delete && c.APIToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.APIToken)
		return nil
	}
	c.Log.Info("No API token, or performing delete operation")
	if c.APIKey == "" || c.APIEmail == "" {
		err := fmt.Errorf("apiKey or apiEmail not found")
		c.Log.Error(err, "Trying to perform Delete request, or any other request with out APIToken, cannot find API Key or Email")
		return err
	}
	req.Header.Add("X-Auth-Key", c.APIKey)
	req.Header.Add("X-Auth-Email", c.APIEmail)
	return nil
}

func (c *CloudflarAPI) CreateCloudflareTunnel() (string, string, error) {
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

	// Generate body for POST request
	postBody, _ := json.Marshal(map[string]string{
		"name":          c.TunnelName,
		"tunnel_secret": tunnelSecret,
	})
	reqBody := bytes.NewBuffer(postBody)

	req, _ := http.NewRequest("POST", CLOUDFLARE_ENDPOINT+"accounts/"+c.ValidAccountId+"/tunnels", reqBody)
	if err := c.addAuthHeader(req, false); err != nil {
		return "", "", err
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in creating tunnel")
		return "", "", err
	}

	defer resp.Body.Close()

	var tunnelResponse CloudflareAPITunnelResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelResponse); err != nil {
		c.Log.Error(err, "could not read body in creating tunnel")
		return "", "", err
	}

	if !tunnelResponse.Success {
		err := fmt.Errorf("%v", tunnelResponse.Errors)
		c.Log.Error(err, "received error in creating tunnel")
		return "", "", err
	}

	c.ValidTunnelId = tunnelResponse.Result.Id
	c.ValidTunnelName = tunnelResponse.Result.Name

	// Read credentials section and marshal to string
	creds, _ := json.Marshal(tunnelResponse.Result.CredentialsFile)
	return tunnelResponse.Result.Id, string(creds), nil
}

func (c *CloudflarAPI) DeleteCloudflareTunnel() error {
	if err := c.ValidateAll(); err != nil {
		c.Log.Error(err, "Error in validation")
		return err
	}

	req, _ := http.NewRequest("DELETE", CLOUDFLARE_ENDPOINT+"accounts/"+c.ValidAccountId+"/tunnels/"+url.QueryEscape(c.ValidTunnelId), nil)
	if err := c.addAuthHeader(req, true); err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in deleting tunnel", "tunnelId", c.TunnelId)
		return err
	}

	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// //Convert the body to type string
	// sb := string(body)
	// c.Log.Info("BODYYYYYYYYYYYYYYY!!!!!!!!!!!!!!!!!", "body", sb)

	defer resp.Body.Close()
	var tunnelResponse CloudflareAPIIdResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelResponse); err != nil {
		c.Log.Error(err, "could not read body in deleting tunnel", "tunnelId", c.TunnelId)
		return err
	}

	if !tunnelResponse.Success {
		c.Log.Error(err, "failed to delete tunnel", "tunnelId", c.TunnelId, "tunnelResponse", tunnelResponse)
		return err
	}

	return nil
}

func (c *CloudflarAPI) ValidateAll() error {
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

func (c *CloudflarAPI) GetAccountId() (string, error) {
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

func (c CloudflarAPI) validateAccountId() bool {
	if c.AccountId == "" {
		c.Log.Info("Account ID not provided")
		return false
	}
	req, _ := http.NewRequest("GET", CLOUDFLARE_ENDPOINT+"accounts/"+url.QueryEscape(c.AccountId), nil)
	if err := c.addAuthHeader(req, false); err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting account by Account ID", "accountId", c.AccountId)
		return false
	}

	defer resp.Body.Close()
	var accountResponse CloudflareAPIIdResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResponse); err != nil {
		c.Log.Error(err, "could not read body in getting account by Account ID", "accountId", c.AccountId)
		return false
	}

	return accountResponse.Success && accountResponse.Result.Id == c.AccountId
}

func (c *CloudflarAPI) getAccountIdByName() (string, error) {
	req, _ := http.NewRequest("GET", CLOUDFLARE_ENDPOINT+"accounts?name="+url.QueryEscape(c.AccountName), nil)
	if err := c.addAuthHeader(req, false); err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting account, check accountName", "accountName", c.AccountName)
		return "", err
	}

	defer resp.Body.Close()
	var accountResponse CloudflareAPINameResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResponse); err != nil || !accountResponse.Success {
		c.Log.Error(err, "could not read body in getting account, check accountName", "accountName", c.AccountName)
		return "", err
	}

	switch len(accountResponse.Result) {
	case 0:
		err := fmt.Errorf("no account in response")
		c.Log.Error(err, "found no account, check accountName", "accountName", c.AccountName)
		return "", err
	case 1:
		return accountResponse.Result[0].Id, nil
	default:
		err := fmt.Errorf("more than one account in response")
		c.Log.Error(err, "found more than one account, check accountName", "accountName", c.AccountName)
		return "", err
	}
}

func (c *CloudflarAPI) GetTunnelId() (string, error) {
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
	} else {
		c.Log.Info("Tunnel ID failed, falling back to Tunnel Name")
		tunnelIdFromName, err := c.getTunnelIdByName()
		if err != nil {
			return "", fmt.Errorf("error fetching Tunnel ID by Tunnel Name")
		}
		c.ValidTunnelId = tunnelIdFromName
		c.ValidTunnelName = c.TunnelName
	}
	return c.ValidTunnelId, nil
}

func (c *CloudflarAPI) validateTunnelId() bool {
	if c.TunnelId == "" {
		c.Log.Info("Tunnel ID not provided")
		return false
	}

	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error code in getting account ID")
		return false
	}

	req, _ := http.NewRequest("GET", CLOUDFLARE_ENDPOINT+"accounts/"+c.ValidAccountId+"/tunnels/"+url.QueryEscape(c.TunnelId), nil)
	if err := c.addAuthHeader(req, false); err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting tunnel by Tunnel ID", "tunnelId", c.TunnelId)
		return false
	}

	defer resp.Body.Close()
	var tunnelResponse CloudflareAPIIdResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelResponse); err != nil {
		c.Log.Error(err, "could not read body in getting tunnel by Tunnel ID", "tunnelId", c.TunnelId)
		return false
	}

	c.ValidTunnelName = tunnelResponse.Result.Name

	return tunnelResponse.Success && tunnelResponse.Result.Id == c.TunnelId
}

func (c *CloudflarAPI) getTunnelIdByName() (string, error) {
	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error code in getting account ID")
		return "", err
	}

	req, _ := http.NewRequest("GET", CLOUDFLARE_ENDPOINT+"accounts/"+c.ValidAccountId+"/tunnels?name="+url.QueryEscape(c.TunnelName), nil)
	if err := c.addAuthHeader(req, false); err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	}

	defer resp.Body.Close()
	var tunnelResponse CloudflareAPINameResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelResponse); err != nil || !tunnelResponse.Success {
		c.Log.Error(err, "could not read body in getting tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	}

	switch len(tunnelResponse.Result) {
	case 0:
		err := fmt.Errorf("no tunnel in response")
		c.Log.Error(err, "found no tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	case 1:
		c.ValidTunnelName = tunnelResponse.Result[0].Name
		return tunnelResponse.Result[0].Id, nil
	default:
		err := fmt.Errorf("more than one tunnel in response")
		c.Log.Error(err, "found more than one tunnel, check tunnelName", "tunnelName", c.TunnelName)
		return "", err
	}
}

func (c *CloudflarAPI) GetTunnelCreds(tunnelSecret string) (string, error) {
	if _, err := c.GetAccountId(); err != nil {
		c.Log.Error(err, "error code in getting account ID")
		return "", err
	}

	if _, err := c.GetTunnelId(); err != nil {
		c.Log.Error(err, "error code in getting tunnel ID")
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

func (c *CloudflarAPI) GetZoneId() (string, error) {
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

func (c *CloudflarAPI) getZoneIdByName() (string, error) {
	req, _ := http.NewRequest("GET", CLOUDFLARE_ENDPOINT+"zones/?name="+url.QueryEscape(c.Domain), nil)
	if err := c.addAuthHeader(req, false); err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting zoneId, check domain", "domain", c.Domain)
		return "", err
	}

	defer resp.Body.Close()
	var zoneResponse CloudflareAPINameResponse
	if err := json.NewDecoder(resp.Body).Decode(&zoneResponse); err != nil || !zoneResponse.Success {
		c.Log.Error(err, "could not read body in getting zoneId, check domain", "domain", c.Domain)
		return "", err
	}

	switch len(zoneResponse.Result) {
	case 0:
		err := fmt.Errorf("no zone in response")
		c.Log.Error(err, "found no zone, check domain", "domain", c.Domain, "zoneResponse", zoneResponse)
		return "", err
	case 1:
		return zoneResponse.Result[0].Id, nil
	default:
		err := fmt.Errorf("more than one zone in response")
		c.Log.Error(err, "found more than one zone, check domain", "domain", c.Domain)
		return "", err
	}
}
