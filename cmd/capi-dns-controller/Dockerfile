# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.13.12 AS build
WORKDIR /build

COPY go.mod go.sum /build/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY apis/ apis/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/capi-dns-controller ./cmd/capi-dns-controller/

FROM scratch AS final
COPY --from=build /bin/capi-dns-controller /bin/capi-dns-controller
ENTRYPOINT ["/bin/capi-dns-controller"]
