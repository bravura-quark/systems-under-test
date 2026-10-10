package sut.experiment;

import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

final class DownstreamClient {
    private final HttpClient client;
    private final URI endpoint;
    private final Duration timeout;

    DownstreamClient(String endpoint, Duration timeout) {
        this.endpoint = URI.create(endpoint);
        this.timeout = timeout;
        this.client = HttpClient.newBuilder()
                .version(HttpClient.Version.HTTP_1_1)
                .connectTimeout(timeout)
                .build();
    }

    Result fetch(int delayMs) throws IOException, InterruptedException {
        URI target = withDelay(delayMs);
        HttpRequest request = HttpRequest.newBuilder(target)
                .timeout(timeout)
                .header("Accept", "application/json")
                .GET()
                .build();
        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        return new Result(response.statusCode(), response.body());
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
