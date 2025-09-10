# Build stage
FROM golang:1.22 AS builder

WORKDIR /app

# Copy all source code including vendor directory
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o slick-autobuild main.go

# Runtime stage - use distroless for security and small size
FROM gcr.io/distroless/static-debian12

# Copy the binary
COPY --from=builder /app/slick-autobuild /slick-autobuild

ENTRYPOINT ["/slick-autobuild"]