# Start the Go app build
FROM golang:latest AS build

# Copy source
WORKDIR /app
COPY . .

# Get required modules (assumes packages have been added to ./vendor)
# RUN go mod download
RUN go mod tidy

# Build a statically-linked Go binary for Linux
RUN CGO_ENABLED=0 GOOS=linux go build -a -o main .

# New build phase -- create binary-only image
FROM alpine:latest

# Add support for HTTPS
RUN apk update && \
    apk upgrade && \
    apk add ca-certificates

WORKDIR /

# Copy files from previous build container
COPY --from=build /app/main ./

# Add environment variables
ENV LOGGLY_TOKEN=3978ab6c-18d0-4709-8d65-38a8b73f88a3

# Expose 8080
EXPOSE 8080

# Check results
RUN env && pwd && find .

# Start the application
CMD ["./main"]