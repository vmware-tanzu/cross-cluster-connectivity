# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

ARG GOVERSION=latest
FROM golang:$GOVERSION AS build
WORKDIR /build

COPY go.mod go.sum /build/
COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/dns-config-patcher ./cmd/dns-config-patcher/

FROM scratch AS final
COPY --from=build /bin/dns-config-patcher /bin/dns-config-patcher
ENTRYPOINT ["/bin/dns-config-patcher"]
