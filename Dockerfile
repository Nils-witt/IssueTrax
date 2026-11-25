# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:26-alpine AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY docs/embed.go docs/openapi.yaml ./docs/
COPY frontend/embed.go ./frontend/embed.go
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X nilswitt.dev/issuetrax/internal/version.Version=${VERSION} -X nilswitt.dev/issuetrax/internal/version.Commit=${COMMIT}" \
    -o /out/issuetrax ./cmd/issuetrax

FROM gcr.io/distroless/static-debian13:nonroot
ENV ISSUETRAX_ADDR=:8080
COPY --from=builder /out/issuetrax /issuetrax
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/issuetrax", "healthcheck"]
ENTRYPOINT ["/issuetrax"]
