package sut.experiment;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class ClientVariantTest {
    @Test
    void parsesEveryExplicitClientVariant() {
        assertEquals(ClientVariant.SYNC, ClientVariant.parse("sync"));
        assertEquals(ClientVariant.ASYNC, ClientVariant.parse("async"));
        assertEquals(ClientVariant.SYNC_TUNED, ClientVariant.parse("sync-tuned"));
    }

    @Test
    void rejectsAnUnknownClientVariant() {
        assertThrows(IllegalArgumentException.class, () -> ClientVariant.parse("implicit"));
    }

    @Test
    void exposesVariantAndTransportSettingsAsMetadata() {
        DownstreamClient client = new DownstreamClient(
                "http://127.0.0.1:1/io", Duration.ofMillis(500), ClientVariant.ASYNC);

        assertEquals("async", client.metadata().get("client_variant"));
        assertEquals("HTTP_1_1", client.metadata().get("http_version"));
        assertEquals("string", client.metadata().get("response_body_handler"));
        assertEquals("sendAsync.get", client.metadata().get("completion_strategy"));
    }

    @Test
    void tunedVariantDeclaresItsExplicitJdkPoolSettings() {
        DownstreamClient client = new DownstreamClient(
                "http://127.0.0.1:1/io", Duration.ofMillis(500), ClientVariant.SYNC_TUNED);

        assertEquals("100", client.metadata().get("connection_pool_size"));
        assertEquals("30s", client.metadata().get("keepalive_timeout"));
    }
}
