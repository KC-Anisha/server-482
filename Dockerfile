# Start the Go app build
FROM 817615305328.dkr.ecr.us-east-1.amazonaws.com/golang:latest AS build
# FROM golang:latest AS build

# Copy source
WORKDIR /app
COPY . .

# Get required modules (assumes packages have been added to ./vendor)
# RUN go mod download
RUN go mod tidy

# Build a statically-linked Go binary for Linux
RUN CGO_ENABLED=0 GOOS=linux go build -a -o main .

# New build phase -- create binary-only image
FROM 817615305328.dkr.ecr.us-east-1.amazonaws.com/alpine:latest
# FROM alpine:latest

# Add support for HTTPS
RUN apk update && \
    apk upgrade && \
    apk add ca-certificates

WORKDIR /

# Copy files from previous build container
COPY --from=build /app/main ./

# Check results
RUN env && pwd && find .

# Start the application
CMD ["./main"]