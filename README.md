# Cloudflare Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/adyanth/cloudflare-operator)](https://goreportcard.com/report/github.com/adyanth/cloudflare-operator)

> **This is not an official operator provided/backed by Cloudflare Inc.**

> **_NOTE_**: This project is currently in Alpha

## Motivation

The [Cloudflare Tunnels guide](https://developers.cloudflare.com/cloudflare-one/tutorials/many-cfd-one-tunnel) for deployment on Kubernetes provides a [manifest](https://github.com/cloudflare/argo-tunnel-examples/tree/master/named-tunnel-k8s) which is very bare bones and does not hook into Kubernetes in any meaningful way. The operator started out as a hobby project of mine to deploy applications in my home lab and expose them to the internet via Cloudflare Tunnels without doing a lot of manual work every time a new application is deployed.

## Overview

The Cloudflare Operator aims to provide a new way of dynamically deploying the [cloudflared](https://github.com/cloudflare/cloudflared) daemon on Kubernetes. Scaffolded and built using `operator-sdk`. Once deployed, this operator provides the following:

* Ability to create new and use existing Tunnels for [Cloudflare for Teams](https://developers.cloudflare.com/cloudflare-one/) using Custom Resources (CR/CRD) which will:
    * Accept a Secret for Cloudflare API Tokens and Keys
    * Run a scaled (configurable) Deployment of `cloudflared`
    * Manage a ConfigMap for the above Deployment
* A Service controller which monitors Service Resources for Annotations and do the following:
    * Update the `cloudflared` ConfigMap to include the new Service to be served
    * Restart the `cloudflared` Deployment to make the configuration change take effect
    * Add a DNS entry in Cloudflare for the specified domain to be a [proxied CNAME to the referenced tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/routing-to-tunnel/dns)
    * Reverse the above when the Service is deleted using Finalizers

## Getting Started

Go through the dedicated documentation on [Getting Started](docs/getting-started.md) to learn how to deploy this operator and a sample tunnel along with a service to expose.

Look into the [Configuration](docs/configuration.md) documentation to understand various configurable parameters of this operator.
