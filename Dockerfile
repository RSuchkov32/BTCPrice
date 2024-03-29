# Start from the latest golang base image
FROM golang:latest

# Add Maintainer Info
LABEL maintainer="Roman"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Copy /Server
COPY /Server /Server

# Copy /UnitTests
COPY /UnitTests /UnitTests

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main .

# Expose port 50051 to the outside world
EXPOSE 50051

# Command to run the executable
CMD ["./main"]