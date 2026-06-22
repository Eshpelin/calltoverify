# --- build ---
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/coordinator ./cmd/coordinator

# --- run ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/coordinator /coordinator
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/coordinator"]
