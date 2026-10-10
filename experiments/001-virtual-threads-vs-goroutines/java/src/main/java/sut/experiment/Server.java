package sut.experiment;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.time.Duration;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class Server {
    private Server() {}

    public static void main(String[] args) throws IOException {
        int port = environmentInt("PORT", 8080);
        DownstreamClient downstreamClient = new DownstreamClient(
                environmentString("DOWNSTREAM_URL", "http://downstream:8080/io"),
                Duration.ofMillis(environmentInt("DOWNSTREAM_TIMEOUT_MS", 500)));
        HttpServer server = HttpServer.create(new InetSocketAddress("0.0.0.0", port), 256);
        ExecutorService executor = Executors.newThreadPerTaskExecutor(
                Thread.ofVirtual().name("sut-java-", 0).factory());
        server.setExecutor(executor);
        server.createContext("/health", Server::health);
        server.createContext("/io", Server::io);
        server.createContext("/downstream", exchange -> downstream(exchange, downstreamClient));

        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            server.stop(0);
            executor.shutdownNow();
        }, "sut-java-shutdown"));

        server.start();
        System.out.println("java service listening on port " + port);
    }

    private static void health(HttpExchange exchange) throws IOException {
        if (!"GET".equals(exchange.getRequestMethod())) {
            try {
                Api.write(exchange, new Api.Response(405, "{\"error\":\"method not allowed\"}"));
            } finally {
                exchange.close();
            }
            return;
        }
        try {
            Api.write(exchange, Api.health());
        } finally {
            exchange.close();
        }
    }

    private static void io(HttpExchange exchange) throws IOException {
        if (!"GET".equals(exchange.getRequestMethod())) {
            try {
                Api.write(exchange, new Api.Response(405, "{\"error\":\"method not allowed\"}"));
            } finally {
                exchange.close();
            }
            return;
        }

        int delayMs;
        try {
            delayMs = Api.parseDelay(Api.queryValue(exchange.getRequestURI().getRawQuery(), "delay"));
        } catch (IllegalArgumentException exception) {
            Api.write(exchange, Api.badRequest(exception.getMessage()));
            exchange.close();
            return;
        }

        try {
            long waitMs = Math.min(delayMs, Api.REQUEST_TIMEOUT.toMillis());
            Api.awaitDelay(Duration.ofMillis(waitMs));
            if (delayMs > Api.REQUEST_TIMEOUT.toMillis()) {
                Api.write(exchange, Api.timeout(delayMs));
            } else {
                Api.write(exchange, Api.ioSuccess(delayMs));
            }
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            // The request was cancelled while waiting. Do not write a success response.
        } finally {
            exchange.close();
        }
    }

    private static void downstream(HttpExchange exchange, DownstreamClient client) throws IOException {
        if (!"GET".equals(exchange.getRequestMethod())) {
            try {
                Api.write(exchange, new Api.Response(405, "{\"error\":\"method not allowed\"}"));
            } finally {
                exchange.close();
            }
            return;
        }

        int delayMs;
        try {
            delayMs = Api.parseDelay(Api.queryValue(exchange.getRequestURI().getRawQuery(), "delay"));
        } catch (IllegalArgumentException exception) {
            Api.write(exchange, Api.badRequest(exception.getMessage()));
            exchange.close();
            return;
        }

        try {
            DownstreamClient.Result result = client.fetch(delayMs);
            if (result.statusCode() < 200 || result.statusCode() >= 300) {
                Api.write(exchange, Api.badGateway(delayMs));
            } else {
                Api.write(exchange, Api.ioSuccess(delayMs));
            }
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            // The request was cancelled while waiting. Do not write a success response.
        } catch (IOException | IllegalArgumentException failure) {
            Api.write(exchange, Api.badGateway(delayMs));
        } finally {
            exchange.close();
        }
    }

    private static int environmentInt(String name, int defaultValue) {
        String value = System.getenv(name);
        if (value == null || value.isBlank()) {
            return defaultValue;
        }
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException exception) {
            throw new IllegalArgumentException(name + " must be an integer", exception);
        }
    }

    private static String environmentString(String name, String defaultValue) {
        String value = System.getenv(name);
        return value == null || value.isBlank() ? defaultValue : value;
    }
}
