# Dockerfile for GoReleaser builds
# This is a minimal Dockerfile optimized for GoReleaser

FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates git

# Create non-root user
RUN adduser -D -h /home/wut wut

# Copy binary (GoReleaser will replace this)
COPY wut /usr/local/bin/wut
RUN chmod +x /usr/local/bin/wut

# Switch to non-root user
USER wut
WORKDIR /home/wut

# Create config directories
RUN mkdir -p /home/wut/.config/wut /home/wut/.wut

# Set entrypoint
ENTRYPOINT ["wut"]
CMD ["--help"]
