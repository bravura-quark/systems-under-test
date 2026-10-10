FROM gradle:9.8.1-jdk25 AS build

WORKDIR /workspace/java
COPY java/ ./
RUN gradle --no-daemon clean jar

FROM golang:1.26.0-bookworm AS healthcheck

WORKDIR /workspace/healthcheck
COPY infrastructure/healthcheck/main.go ./main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" go build -trimpath -ldflags='-s -w' -o /healthcheck /workspace/healthcheck/main.go

FROM eclipse-temurin:25.0.4.1_1-jre-ubi10-minimal

WORKDIR /app
COPY --from=build /workspace/java/build/libs/java-virtual-threads-service-0.1.0.jar /app/app.jar
COPY --from=healthcheck /healthcheck /usr/local/bin/healthcheck

USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=1s --start-period=5s --retries=10 CMD ["/usr/local/bin/healthcheck", "http://127.0.0.1:8080/health"]
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
