FROM golang:1.26.0-bookworm AS build

ENV CGO_ENABLED=0
RUN GOBIN=/out go install -ldflags="-X main.Version=12.13.0 -X main.Commit=v12.13.0" github.com/tsenart/vegeta/v12@v12.13.0

FROM scratch

COPY --from=build /out/vegeta /vegeta
ENTRYPOINT ["/vegeta"]
