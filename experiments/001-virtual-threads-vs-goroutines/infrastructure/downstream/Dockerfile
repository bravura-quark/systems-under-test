FROM golang:1.26.0-bookworm AS build

WORKDIR /workspace/downstream
COPY infrastructure/downstream/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /downstream ./cmd/server

WORKDIR /workspace/healthcheck
COPY infrastructure/healthcheck/main.go ./main.go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /healthcheck ./main.go

FROM scratch

COPY --from=build /downstream /downstream
COPY --from=build /healthcheck /healthcheck

EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=1s --start-period=1s --retries=30 CMD ["/healthcheck", "http://127.0.0.1:8080/health"]
ENTRYPOINT ["/downstream"]
