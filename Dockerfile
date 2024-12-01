# Use the official Golang image version 1.23 with the Alpine distribution as the base image for the build stage.
# This multi-stage build starts with the "build-stage" stage where the Go application will be compiled.
FROM golang:1.23.1-alpine3.20 AS build-stage

# Perform system updates and install necessary packages.
# - `apk --no-cache update`: Updates the Alpine package repository without caching index files.
# - `apk --no-cache upgrade`: Upgrades all installed packages to the latest available versions.
# - `apk --no-cache add`: Installs additional required packages:
#   - `ca-certificates`: For managing CA certificates for secure communication.
#   - `curl`: For making HTTP requests (can be used to download files or for health checks).
#   - `gcc` and `g++`: The GNU Compiler Collection used for compiling C and C++ code, essential for building Go applications.
#   - `git`: Required for downloading Go modules that reference external repositories.
#   - `make`: Utility for automating build processes and running the `Makefile`.
RUN <<EOF
    apk --no-cache update
    apk --no-cache upgrade

    apk --no-cache add ca-certificates curl gcc g++ git make
EOF

# Set the working directory inside the container to `/xurlfind3r`.
# All subsequent commands (COPY, RUN, etc.) will operate relative to this directory.
WORKDIR /xurlfind3r

# Copy the Go module files (`go.mod` and `go.sum`) from the host to the container.
# This allows Docker to cache the dependency downloads, avoiding repeated downloads unless these files change.
COPY go.mod go.sum ./

# Download the Go dependencies specified in `go.mod` and `go.sum`.
# This ensures that the environment has all the required libraries before building the application.
RUN go mod download

# Copy the entire source code from the host machine to the working directory inside the container.
# This includes all files from the current directory where the Dockerfile is located into the `/xurlfind3r` directory.
COPY . .

# Run the `make go-build` command to build the Go application.
# Assumes a `Makefile` exists in the project root that handles the build process (likely using `go build`).
RUN make go-build

# Start the second stage of the multi-stage build using the `alpine:3.20.3` image.
# This stage is designed to produce a much smaller, production-ready image containing only the necessary runtime components.
FROM alpine:3.20.3

# Perform system updates and install essential runtime packages:
# - `bind-tools`: Provides DNS lookup utilities like `dig` and `host`.
# - `ca-certificates`: Includes CA certificates to allow HTTPS connections.
# Create a non-root user and group to enhance the security of the running container.
# Files and processes will be owned and run by this user instead of the root user.
RUN <<EOF
    apk --no-cache update
    apk --no-cache upgrade

    apk --no-cache add bind-tools ca-certificates

    addgroup runners

    adduser runner -D -G runners
EOF

# Switch to the non-root user. All subsequent commands will use this unprivileged user.
USER runner

# Copy the compiled binary `xurlfind3r` from the `build-stage` stage.
# The binary is located at `/xurlfind3r/bin/xurlfind3r` in the build environment and is copied to `/usr/local/bin/` in the final image.
COPY --from=build-stage /xurlfind3r/bin/xurlfind3r /usr/local/bin/

# Set the default command to execute the `xurlfind3r` binary when the container starts.
# This means that when you run the container, it will automatically start the `xurlfind3r` tool.
ENTRYPOINT ["xurlfind3r"]