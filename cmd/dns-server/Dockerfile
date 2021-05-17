# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

ARG GOVERSION=latest
FROM golang:$GOVERSION AS build
WORKDIR /build

COPY go.mod go.sum /build/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY apis/ apis/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/dns-server ./cmd/dns-server/

FROM scratch AS final
COPY --from=build /bin/dns-server /bin/dns-server
ENTRYPOINT ["/bin/dns-server"]
