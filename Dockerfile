#########################################################
# Build the sources and provide the result in a multi stage
# docker container. The alpine build image has to match
# the alpine image in the referencing runtime container.
#########################################################
FROM golang:1.11.13-alpine3.10 AS builder

RUN apk --no-cache add git

# Directory in workspace
WORKDIR "/go/src/github.com/Peripli/service-broker-proxy-cf"

# Copy and build source code
ENV GO111MODULE=on
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /main main.go

########################################################
# Build the runtime container
########################################################
FROM alpine:3.10 AS package_step

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy the executable file
COPY --from=builder /main /app/
COPY --from=builder /go/src/github.com/Peripli/service-broker-proxy-cf/application.yml /app/

EXPOSE 8080
ENTRYPOINT [ "./main" ]
