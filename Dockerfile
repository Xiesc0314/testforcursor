FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/proxy ./cmd/proxy

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/proxy /proxy
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/proxy"]

