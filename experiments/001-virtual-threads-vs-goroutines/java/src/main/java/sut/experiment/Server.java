package sut.experiment;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.time.Duration;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import java.util.concurrent.Semaphore;
import java.util.ArrayList;
import java.util.List;

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
        ExecutorService fanoutExecutor = Executors.newVirtualThreadPerTaskExecutor();
        Semaphore fanoutAdmission = new Semaphore(
                Math.max(1, environmentInt("FANOUT_MAX_IN_FLIGHT", 256)));
        Semaphore fanoutBranchAdmission = new Semaphore(
                Math.max(1, environmentInt("FANOUT_MAX_BRANCHES_IN_FLIGHT", 256)));
        server.setExecutor(executor);
        server.createContext("/health", Server::health);
        server.createContext("/io", Server::io);
        server.createContext("/downstream", exchange -> downstream(exchange, downstreamClient));
        server.createContext("/fanout", exchange -> fanout(
                exchange, downstreamClient, fanoutExecutor, fanoutAdmission, fanoutBranchAdmission));

        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            server.stop(0);
            executor.shutdownNow();
            fanoutExecutor.shutdownNow();
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

    static void fanout(HttpExchange exchange, DownstreamClient client) throws IOException {
        try (ExecutorService branchExecutor = Executors.newVirtualThreadPerTaskExecutor()) {
            fanout(exchange, client, branchExecutor);
        }
    }

    static void fanout(HttpExchange exchange, DownstreamClient client, ExecutorService branchExecutor)
            throws IOException {
        fanout(exchange, client, branchExecutor, new Semaphore(Integer.MAX_VALUE));
    }

    static void fanout(
            HttpExchange exchange,
            DownstreamClient client,
            ExecutorService branchExecutor,
            Semaphore admission)
            throws IOException {
        fanout(exchange, client, branchExecutor, admission, new Semaphore(Integer.MAX_VALUE));
    }

    static void fanout(
            HttpExchange exchange,
            DownstreamClient client,
            ExecutorService branchExecutor,
            Semaphore admission,
            Semaphore branchAdmission)
            throws IOException {
        if (!"GET".equals(exchange.getRequestMethod())) {
            try {
                Api.write(exchange, new Api.Response(405, "{\"error\":\"method not allowed\"}"));
            } finally {
                exchange.close();
            }
            return;
        }

        int delayMs;
        int branches;
        try {
            delayMs = Api.parseDelay(Api.queryValue(exchange.getRequestURI().getRawQuery(), "delay"));
            branches = Api.parseBranches(Api.queryValue(exchange.getRequestURI().getRawQuery(), "branches"));
        } catch (IllegalArgumentException exception) {
            try {
                Api.write(exchange, Api.badRequest(exception.getMessage()));
            } finally {
                exchange.close();
            }
            return;
        }

        if (!admission.tryAcquire()) {
            try {
                Api.write(exchange, new Api.Response(503,
                        "{\"status\":\"capacity\",\"delay_ms\":" + delayMs + "}"));
            } finally {
                exchange.close();
            }
            return;
        }

        if (!branchAdmission.tryAcquire(branches)) {
            admission.release();
            try {
                Api.write(exchange, new Api.Response(503,
                        "{\"status\":\"capacity\",\"delay_ms\":" + delayMs + "}"));
            } finally {
                exchange.close();
            }
            return;
        }

        try {
            List<Future<DownstreamClient.Result>> futures = new ArrayList<>(branches);
            boolean successful = true;
            try {
                for (int branch = 0; branch < branches; branch++) {
                    futures.add(branchExecutor.submit(() -> client.fetch(delayMs)));
                }
                for (Future<DownstreamClient.Result> future : futures) {
                    DownstreamClient.Result result = future.get();
                    if (result.statusCode() < 200 || result.statusCode() >= 300) {
                        successful = false;
                        break;
                    }
                }
            } catch (InterruptedException interrupted) {
                successful = false;
                cancelAll(futures);
                Thread.currentThread().interrupt();
                return;
            } catch (ExecutionException | RuntimeException failure) {
                successful = false;
            } finally {
                if (!successful) {
                    cancelAll(futures);
                }
            }
            Api.write(exchange, successful ? Api.ioSuccess(delayMs) : Api.badGateway(delayMs));
        } finally {
            branchAdmission.release(branches);
            admission.release();
            exchange.close();
        }
    }

    private static void cancelAll(List<? extends Future<?>> futures) {
        for (Future<?> future : futures) {
            future.cancel(true);
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
