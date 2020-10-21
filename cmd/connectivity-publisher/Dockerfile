# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.13.12 AS build
WORKDIR /build

COPY go.mod go.sum /build/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY apis/ apis/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/connectivity-publisher ./cmd/connectivity-publisher/

FROM scratch AS final
COPY --from=build /bin/connectivity-publisher /bin/connectivity-publisher
ENTRYPOINT ["/bin/connectivity-publisher"]
