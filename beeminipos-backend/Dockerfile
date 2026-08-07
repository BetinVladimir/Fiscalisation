FROM golang:1.24 AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/beeminipos-backend ./cmd/beeminipos-backend

FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/beeminipos-backend /beeminipos-backend
EXPOSE 8081
ENTRYPOINT ["/beeminipos-backend"]
