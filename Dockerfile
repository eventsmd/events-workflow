FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/events-workflow ./cmd/events-workflow

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/events-workflow /events-workflow
EXPOSE 8080
ENTRYPOINT ["/events-workflow"]
