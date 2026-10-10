package sut.experiment;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.time.Duration;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

class ApiTest {
    @Test
    void parsesTheBaselineDelay() {
        assertEquals(50, Api.parseDelay("50"));
    }

    @Test
    void rejectsMissingMalformedNegativeAndExcessiveDelays() {
        assertThrows(IllegalArgumentException.class, () -> Api.parseDelay(null));
        assertThrows(IllegalArgumentException.class, () -> Api.parseDelay("not-a-number"));
        assertThrows(IllegalArgumentException.class, () -> Api.parseDelay("-1"));
        assertThrows(IllegalArgumentException.class, () -> Api.parseDelay("5001"));
    }

    @Test
    void returnsStableResponseShapes() {
        assertEquals("{\"status\":\"ok\"}", Api.health().body());
        assertEquals("{\"status\":\"ok\",\"delay_ms\":50}", Api.ioSuccess(50).body());
        assertEquals(400, Api.badRequest("invalid delay").status());
        assertTrue(Api.badRequest("invalid delay").body().contains("invalid delay"));
    }

    @Test
    void delayWaitIsInterruptibleAndRestoresInterruptStatus() throws Exception {
        CountDownLatch started = new CountDownLatch(1);
        CountDownLatch interrupted = new CountDownLatch(1);

        Thread worker = Thread.ofVirtual().start(() -> {
            started.countDown();
            try {
                Api.awaitDelay(Duration.ofSeconds(5));
            } catch (InterruptedException expected) {
                if (Thread.currentThread().isInterrupted()) {
                    interrupted.countDown();
                }
            }
        });

        assertTrue(started.await(1, TimeUnit.SECONDS));
        worker.interrupt();
        assertTrue(interrupted.await(1, TimeUnit.SECONDS));
        worker.join();
    }
}
