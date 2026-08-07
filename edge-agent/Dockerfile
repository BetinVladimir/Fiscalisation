FROM golang:1.26.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/edge-agent ./cmd/edge-agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/edge-agent /edge-agent
VOLUME ["/var/lib/beefiscal-edge"]
EXPOSE 8082
ENTRYPOINT ["/edge-agent"]
