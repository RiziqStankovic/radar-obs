FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/radar-gateway ./cmd/radar-gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/radar-gateway /radar-gateway
EXPOSE 4318
ENTRYPOINT ["/radar-gateway"]
