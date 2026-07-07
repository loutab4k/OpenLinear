FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/openlinear ./cmd/openlinear && mkdir /out/config

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/openlinear /usr/local/bin/openlinear
# Same binary under the short name, so `docker compose run openlinear` and
# docs can both say `ol ...`.
COPY --from=build /out/openlinear /usr/local/bin/ol
# Pre-create the credentials dir owned by nonroot: the named `config` volume
# inherits this ownership on first mount, so `auth login` can write to it.
COPY --from=build --chown=nonroot:nonroot /out/config /config
USER nonroot:nonroot
ENTRYPOINT ["openlinear"]
CMD ["run", "--data-dir", "/data"]
