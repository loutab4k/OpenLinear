FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/openlinear ./cmd/openlinear

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/openlinear /usr/local/bin/openlinear
USER nonroot:nonroot
ENTRYPOINT ["openlinear"]
CMD ["run", "--data-dir", "/data"]
