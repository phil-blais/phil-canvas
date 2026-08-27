# Multi-stage build for the Go backend. Produces a small static image suitable
# for Cloud Run.

# --- build stage ---
FROM golang:1.25 AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static, stripped binary. CGO off so it runs on the distroless static image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/backend .

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/backend /backend
# Cloud Run sets PORT; default for local `docker run`.
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/backend"]
