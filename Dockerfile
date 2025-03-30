FROM golang:1.24 AS build

RUN useradd -u 10001 dimo

WORKDIR /build
COPY . ./

RUN make tidy
RUN make build

FROM gcr.io/distroless/static AS final

LABEL maintainer="DIMO <hello@dimo.zone>"

USER nonroot:nonroot

COPY --from=build --chown=nonroot:nonroot /build/bin/fetch-api /

EXPOSE 8080
EXPOSE 8888

ENTRYPOINT ["/fetch-api"]

