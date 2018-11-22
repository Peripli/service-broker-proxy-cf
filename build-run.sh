rm -rf vendor/github.com/Peripli/service-manager/pkg
rm -rf vendor/github.com/Peripli/service-manager/api

cp -R ../service-manager/pkg vendor/github.com/Peripli/service-manager/
cp -R ../service-manager/api vendor/github.com/Peripli/service-manager/

rm -rf vendor/github.com/Peripli/service-broker-proxy/pkg
cp -R ../service-broker-proxy/pkg vendor/github.com/Peripli/service-broker-proxy/

go run main.go