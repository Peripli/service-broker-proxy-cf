# service-broker-proxy-cf

[![Build Status](https://travis-ci.org/Peripli/service-broker-proxy-cf.svg?branch=master)](https://travis-ci.org/Peripli/service-broker-proxy-cf)
[![Go Report Card](https://goreportcard.com/badge/github.com/Peripli/service-broker-proxy-cf)](https://goreportcard.com/report/github.com/Peripli/service-broker-proxy-cf)
[![Coverage Status](https://coveralls.io/repos/github/Peripli/service-broker-proxy-cf/badge.svg?branch=master)](https://coveralls.io/github/Peripli/service-broker-proxy-cf)

CF Specific Implementation for Service Broker Proxy Module

## Installation of the service broker proxy on CF
Modify the manifest file *manifest.yml*. Replace the service manager address and credentials for the platform accordingly.

```
      SM_HOST: https://service-manager.com
      CF_USERNAME: cfuser
      CF_PASSWORD: cfpassword
```

Where:
* SM_HOST is the host of the Service Manager installation that shall be used.
* CF_USERNAME/CF_PASSWORD are the credentials used for uthorisation and communication with cloud controller. 

## Deployment on your kubernetes cluster
```
cf push -f manifest.yml
```
