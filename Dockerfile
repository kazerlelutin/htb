FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/htbd ./cmd/htbd && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/htb ./cmd/htb

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/htbd /usr/local/bin/htbd
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/htbd"]
