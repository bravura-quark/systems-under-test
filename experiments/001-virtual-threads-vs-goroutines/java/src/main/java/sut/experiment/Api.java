package sut.experiment;

import com.sun.net.httpserver.HttpExchange;
import java.io.IOException;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.time.Duration;

final class Api {
    static final int MAX_DELAY_MS = 5_000;
    static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(2);

    private Api() {}

    record Response(int status, String body) {}

    static int parseDelay(String rawDelay) {
        if (rawDelay == null || rawDelay.isBlank()) {
            throw new IllegalArgumentException("delay is required");
        }
        final int delay;
        try {
            delay = Integer.parseInt(rawDelay);
        } catch (NumberFormatException exception) {
            throw new IllegalArgumentException("delay must be an integer", exception);
        }
        if (delay < 0 || delay > MAX_DELAY_MS) {
            throw new IllegalArgumentException("delay must be between 0 and " + MAX_DELAY_MS + " ms");
        }
        return delay;
    }

    static Response health() {
        return new Response(200, "{\"status\":\"ok\"}");
    }

    static Response ioSuccess(int delayMs) {
        return new Response(200, "{\"status\":\"ok\",\"delay_ms\":" + delayMs + "}");
    }

    static Response timeout(int delayMs) {
        return new Response(504, "{\"status\":\"timeout\",\"delay_ms\":" + delayMs + "}");
    }

    static Response badGateway(int delayMs) {
        return new Response(502, "{\"status\":\"bad_gateway\",\"delay_ms\":" + delayMs + "}");
    }

    static Response badRequest(String message) {
        return new Response(400, "{\"error\":\"" + message + "\"}");
    }

    static void awaitDelay(Duration delay) throws InterruptedException {
        try {
            Thread.sleep(delay);
        } catch (InterruptedException interrupted) {
            // Thread.sleep clears the flag when it throws; preserve cancellation for callers.
            Thread.currentThread().interrupt();
            throw interrupted;
        }
    }

    static void write(HttpExchange exchange, Response response) throws IOException {
        byte[] body = response.body().getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json; charset=utf-8");
        exchange.getResponseHeaders().set("Content-Length", Integer.toString(body.length));
        exchange.sendResponseHeaders(response.status(), body.length);
        try (var output = exchange.getResponseBody()) {
            output.write(body);
        }
    }

    static String queryValue(String query, String name) {
        if (query == null || query.isBlank()) {
            return null;
        }
        for (String parameter : query.split("&")) {
            String[] keyAndValue = parameter.split("=", 2);
            String key = URLDecoder.decode(keyAndValue[0], StandardCharsets.UTF_8);
            if (name.equals(key)) {
                String value = keyAndValue.length == 2 ? keyAndValue[1] : "";
                return URLDecoder.decode(value, StandardCharsets.UTF_8);
            }
        }
        return null;
    }
}
