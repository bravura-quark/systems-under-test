package sut.experiment;

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class FanoutTest {
    private HttpServer downstream;
    private HttpServer service;

    @BeforeEach
    void startServers() throws IOException {
        downstream = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        service = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    }

    @AfterEach
    void stopServers() {
        service.stop(0);
        downstream.stop(0);
    }

    @Test
    void makesOneDownstreamRequestPerBranch() throws Exception {
        AtomicInteger requests = new AtomicInteger();
        downstream.createContext("/io", exchange -> {
            requests.incrementAndGet();
            exchange.sendResponseHeaders(200, -1);
            exchange.close();
        });
        downstream.start();
        DownstreamClient client = new DownstreamClient(endpoint(downstream, "/io"), Duration.ofSeconds(1));
        service.createContext("/fanout", exchange -> Server.fanout(exchange, client));
        service.start();

        HttpResponse<String> response = request("/fanout?delay=50&branches=4");

        assertEquals(200, response.statusCode());
        assertEquals("{\"status\":\"ok\",\"delay_ms\":50}", response.body());
        assertEquals(4, requests.get());
    }

    @Test
    void mapsDownstreamFailureToBadGateway() throws Exception {
        downstream.createContext("/io", exchange -> {
            exchange.sendResponseHeaders(503, -1);
            exchange.close();
        });
        downstream.start();
        DownstreamClient client = new DownstreamClient(endpoint(downstream, "/io"), Duration.ofSeconds(1));
        service.createContext("/fanout", exchange -> Server.fanout(exchange, client));
        service.start();

        HttpResponse<String> response = request("/fanout?delay=50&branches=4");

        assertEquals(502, response.statusCode());
        assertEquals("{\"status\":\"bad_gateway\",\"delay_ms\":50}", response.body());
    }

    @Test
    void rejectsInvalidBranchCount() throws Exception {
        service.createContext("/fanout", exchange -> Server.fanout(exchange,
                new DownstreamClient("http://127.0.0.1:1/io", Duration.ofSeconds(1))));
        service.start();

        assertEquals(400, request("/fanout?delay=50&branches=17").statusCode());
    }

    private HttpResponse<String> request(String path) throws Exception {
        return HttpClient.newHttpClient().send(
                HttpRequest.newBuilder(URI.create(endpoint(service, path))).GET().build(),
                HttpResponse.BodyHandlers.ofString());
    }

    private static String endpoint(HttpServer server, String path) {
        return "http://127.0.0.1:" + server.getAddress().getPort() + path;
    }
}
