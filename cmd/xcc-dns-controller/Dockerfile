# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

FROM golang AS build
WORKDIR /build

COPY go.mod go.sum /build/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY apis/ apis/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/xcc-dns-controller ./cmd/xcc-dns-controller/

FROM scratch AS final
COPY --from=build /bin/xcc-dns-controller /bin/xcc-dns-controller
ENTRYPOINT ["/bin/xcc-dns-controller"]
