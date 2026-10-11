package sut.experiment;

import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

final class DownstreamClient {
    private final HttpClient client;
    private final URI endpoint;
    private final Duration timeout;
    private final ClientVariant variant;
    private final ExecutorService asyncExecutor;

    DownstreamClient(String endpoint, Duration timeout) {
        this(endpoint, timeout, ClientVariant.SYNC);
    }

    DownstreamClient(String endpoint, Duration timeout, ClientVariant variant) {
        this.endpoint = URI.create(endpoint);
        this.timeout = timeout;
        this.variant = variant;
        this.asyncExecutor = variant == ClientVariant.ASYNC
                ? Executors.newFixedThreadPool(32, Thread.ofPlatform().daemon().name("http-async-", 0).factory())
                : null;
        HttpClient.Builder builder = HttpClient.newBuilder()
                .version(HttpClient.Version.HTTP_1_1)
                .connectTimeout(timeout);
        if (asyncExecutor != null) {
            builder.executor(asyncExecutor);
        }
        this.client = builder.build();
    }

    Result fetch(int delayMs) throws IOException, InterruptedException {
        URI target = withDelay(delayMs);
        HttpRequest request = HttpRequest.newBuilder(target)
                .timeout(timeout)
                .header("Accept", "application/json")
                .GET()
                .build();
        if (variant == ClientVariant.ASYNC) {
            try {
                HttpResponse<String> response = client.sendAsync(
                        request, HttpResponse.BodyHandlers.ofString()).get();
                return new Result(response.statusCode(), response.body());
            } catch (ExecutionException failure) {
                Throwable cause = failure.getCause();
                if (cause instanceof IOException ioFailure) {
                    throw ioFailure;
                }
                if (cause instanceof RuntimeException runtimeFailure) {
                    throw runtimeFailure;
                }
                throw new IOException("asynchronous HTTP request failed", cause);
            }
        }
        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        return new Result(response.statusCode(), response.body());
    }

    Map<String, String> metadata() {
        return Map.of(
                "client_variant", variant.externalName(),
                "http_version", "HTTP_1_1",
                "response_body_handler", "string",
                "completion_strategy", variant.completionStrategy(),
                "executor", variant.executor(),
                "connection_pool_size", "100",
                "keepalive_timeout", "30s",
                "connection_reuse", "shared-HttpClient");
    }

    void close() {
        if (asyncExecutor != null) {
            asyncExecutor.shutdownNow();
        }
    }

    private URI withDelay(int delayMs) {
        String separator = endpoint.getQuery() == null ? "?" : "&";
        try {
            return new URI(endpoint + separator + "delay=" + delayMs);
        } catch (URISyntaxException exception) {
            throw new IllegalArgumentException("invalid downstream URL", exception);
        }
    }

    record Result(int statusCode, String body) {}
}
