package sut.experiment;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.http.HttpTimeoutException;
import java.time.Duration;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class DownstreamClientTest {
    private HttpServer server;

    @BeforeEach
    void startServer() throws IOException {
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.start();
    }

    @AfterEach
    void stopServer() {
        server.stop(0);
    }

    @Test
    void sendsHttp11RequestWithDelayAndReturnsResponse() throws Exception {
        server.createContext("/io", exchange -> {
            assertEquals("50", exchange.getRequestURI().getQuery().substring("delay=".length()));
            byte[] body = "{\"status\":\"ok\",\"delay_ms\":50}".getBytes();
            exchange.sendResponseHeaders(200, body.length);
            try (var output = exchange.getResponseBody()) {
                output.write(body);
            }
        });

        DownstreamClient client = new DownstreamClient(endpoint(), Duration.ofSeconds(1));
        DownstreamClient.Result result = client.fetch(50);

        assertEquals(200, result.statusCode());
        assertEquals("{\"status\":\"ok\",\"delay_ms\":50}", result.body());
    }

    @Test
    void returnsNonSuccessStatusToCaller() throws Exception {
        server.createContext("/io", exchange -> {
            exchange.sendResponseHeaders(503, -1);
            exchange.close();
        });

        DownstreamClient.Result result = new DownstreamClient(endpoint(), Duration.ofSeconds(1)).fetch(50);

        assertEquals(503, result.statusCode());
    }

    @Test
    void enforcesRequestTimeout() throws Exception {
        server.createContext("/io", exchange -> {
            try {
                Thread.sleep(500);
            } catch (InterruptedException interrupted) {
                Thread.currentThread().interrupt();
            }
            exchange.close();
        });

        Exception failure = assertThrows(Exception.class,
                () -> new DownstreamClient(endpoint(), Duration.ofMillis(20)).fetch(50));
        assertInstanceOf(HttpTimeoutException.class, failure);
    }

    @Test
    void interruptionStopsWaitingForDownstream() throws Exception {
        CountDownLatch started = new CountDownLatch(1);
        server.createContext("/io", exchange -> {
            started.countDown();
            try {
                Thread.sleep(5_000);
            } catch (InterruptedException interrupted) {
                Thread.currentThread().interrupt();
            } finally {
                exchange.close();
            }
        });

        AtomicReference<Throwable> failure = new AtomicReference<>();
        Thread worker = Thread.ofVirtual().start(() -> {
            try {
                new DownstreamClient(endpoint(), Duration.ofSeconds(10)).fetch(50);
            } catch (Throwable throwable) {
                failure.set(throwable);
            }
        });

        assertTrue(started.await(1, TimeUnit.SECONDS));
        worker.interrupt();
        worker.join(1_000);

        assertTrue(!worker.isAlive(), "interrupted virtual thread did not stop");
        assertInstanceOf(InterruptedException.class, failure.get());
    }

    private String endpoint() {
        return "http://127.0.0.1:" + server.getAddress().getPort() + "/io";
    }
}
