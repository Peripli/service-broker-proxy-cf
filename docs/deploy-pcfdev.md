# Deploy Service-Broker-Proxy on PCF Dev

## Prerequisites

* PCF Dev is installed.
* You are logged in PCF Dev.
* `go_buildpack` is installed with support for go version 1.10
* [Service-Manager](https://github.com/Peripli/service-manager) is deployed.

**Note:** The used go buildpack should be named `go_buildpack`.

## Modify manifest.yml

In `manifest.yml` you need to configure the following:

* Service-Manager host using the `SM_HOST` env variable.
* Administrative credentials for PCF Dev with env variables `CF_USERNAME` and `CF_PASSWORD`.

In addition you can change other configurations like log level and log format.
You can also use the `application.yml` file which has lower priority than the Environment variables.

## Push

Execute:

```sh
cf push -f manifest.yml
```
