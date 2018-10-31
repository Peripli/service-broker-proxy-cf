# Deploy Service-Broker-Proxy on PCF Dev

## Prerequisites

* PCF Dev is installed.
* You are logged in PCF Dev.
* [Service-Manager](https://github.com/Peripli/service-manager) is deployed.


## Modify manifest.yml

In `manifest.yml` you need to configure the following:

* Service-Manager URL using the `SM_URL` env variable.
* Administrative credentials for PCF Dev with env variables `CF_USERNAME` and `CF_PASSWORD`.
* Platform credentials for Service Manager with env variables `SM_USER` and `SM_PASSWORD`

In addition you can change other configurations like log level and log format.

## Push

Execute:

```sh
cf push -f manifest.yml
```
