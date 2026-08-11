FROM golang:1.26.3 AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/beeminipos-backend ./cmd/beeminipos-backend

FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/beeminipos-backend /beeminipos-backend
EXPOSE 8081
ENTRYPOINT ["/beeminipos-backend"]
