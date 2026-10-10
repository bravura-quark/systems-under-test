FROM golang:1.26.0-bookworm AS build

WORKDIR /workspace/go
COPY go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM golang:1.26.0-bookworm AS healthcheck

WORKDIR /workspace/healthcheck
COPY infrastructure/healthcheck/main.go ./main.go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /healthcheck /workspace/healthcheck/main.go

FROM scratch

COPY --from=build /server /server
COPY --from=healthcheck /healthcheck /healthcheck

USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=1s --start-period=2s --retries=10 CMD ["/healthcheck", "http://127.0.0.1:8080/health"]
ENTRYPOINT ["/server"]
