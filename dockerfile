# syntax=docker/dockerfile:1

# Legacy-arch Cosmos image (armv6/armv7 only).
# The base image is injected by the workflow:
#   - armv7:  debian:bookworm            (has linux/arm/v7)
#   - armv6:  balenalib/rpi-raspbian:bookworm  (has linux/arm/v6)
ARG BASE_IMAGE=debian:bookworm
FROM ${BASE_IMAGE}

# balenalib base images ship a wrapper entrypoint; disable it so the
# CMD below controls the container process.
ENTRYPOINT []

ARG TARGETPLATFORM
ARG BINARY_NAME=cosmos-armv7

# Set BINARY_NAME based on the TARGETPLATFORM
RUN case "$TARGETPLATFORM" in \
    "linux/arm/v6") BINARY_NAME="cosmos-armv6" ;; \
    *) BINARY_NAME="cosmos-armv7" ;; \
    esac && echo $BINARY_NAME > /binary_name

# This is just to log the platforms (optional)
RUN echo "I am building for $TARGETPLATFORM" > /log

EXPOSE 443 80

VOLUME /config

RUN apt-get update \
    && apt-get install -y ca-certificates openssl fdisk mergerfs snapraid \
       curl unzip wget avahi-daemon avahi-utils samba fuse3 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the respective binary based on the BINARY_NAME
COPY build/cosmos-armv6 build/cosmos-armv7 ./

# Copy other resources
COPY build/* ./
COPY static ./static

# Run the respective binary based on the BINARY_NAME
CMD ["sh", "-c", "./$(cat /binary_name)"]
