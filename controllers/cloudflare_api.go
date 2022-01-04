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

type CloudflarAPI struct {
	Log         logr.Logger
	TunnelName  string
	AccountName string
	Domain      string
	APIKey      string
}

type CloudflareAPIAccountResponse struct {
	Result []struct {
		Id   string
		Name string
	}
}

type CloudflareAPITunnelResponse struct {
	Result struct {
		Id              string
		CredentialsFile map[string]string `json:"credentials_file"`
	}
}

type CloudflareAPITunnelCreate struct {
	Name         string
	TunnelSecret string `json:"tunnel_secret"`
}

func (c CloudflarAPI) CreateCloudflareTunnel() (string, string, error) {
	accountId, err := c.getAccountName()
	if err != nil {
		return "", "", fmt.Errorf("error fetching AccountName")
	}

	// Generate 32 byte random string for tunnel secret
	randSecret := make([]byte, 32)
	if _, err := rand.Read(randSecret); err != nil {
		return "", "", err
	}
	tunnelSecret := base64.StdEncoding.EncodeToString(randSecret)

	id, creds, err := c.createTunnel(accountId, tunnelSecret)
	return id, creds, err
}

func (c CloudflarAPI) getAccountName() (string, error) {
	req, _ := http.NewRequest("GET", "https://api.cloudflare.com/client/v4/accounts?name="+url.QueryEscape(c.AccountName), nil)
	req.Header.Add("Authorization", "Bearer "+c.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error(err, "error code in getting account, check accountName", "accountName", c.AccountName)
		return "", err
	}

	defer resp.Body.Close()
	var accountResponse CloudflareAPIAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResponse); err != nil {
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

func (c CloudflarAPI) createTunnel(accountId, tunnelSecret string) (string, string, error) {
	// Generate body for POST request
	postBody, _ := json.Marshal(map[string]string{
		"name":          c.TunnelName,
		"tunnel_secret": tunnelSecret,
	})
	reqBody := bytes.NewBuffer(postBody)

	req, _ := http.NewRequest("POST", "https://api.cloudflare.com/client/v4/accounts/"+accountId+"/tunnels", reqBody)
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
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

	// Read credentials section and marshal to string
	creds, _ := json.Marshal(tunnelResponse.Result.CredentialsFile)
	return tunnelResponse.Result.Id, string(creds), nil
}
