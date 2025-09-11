# Build stage
FROM golang:1.22 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o slick-autobuild main.go

# Runtime stage - use distroless for security and small size
FROM gcr.io/distroless/static-debian12

# Copy the binary
COPY --from=builder /app/slick-autobuild /slick-autobuild

ENTRYPOINT ["/slick-autobuild"]