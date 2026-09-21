FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/radar-agent ./cmd/radar-agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/radar-agent /radar-agent
EXPOSE 8080
ENTRYPOINT ["/radar-agent"]
