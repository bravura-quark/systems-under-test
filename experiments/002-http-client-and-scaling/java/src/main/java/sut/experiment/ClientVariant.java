package sut.experiment;

import java.util.Locale;

enum ClientVariant {
    SYNC("sync", "send", "virtual-thread caller"),
    ASYNC("async", "sendAsync.get", "bounded-platform-executor"),
    SYNC_TUNED("sync-tuned", "send", "virtual-thread caller");

    private final String externalName;
    private final String completionStrategy;
    private final String executor;

    ClientVariant(String externalName, String completionStrategy, String executor) {
        this.externalName = externalName;
        this.completionStrategy = completionStrategy;
        this.executor = executor;
    }

    static ClientVariant parse(String value) {
        if (value == null || value.isBlank()) {
            return SYNC;
        }
        String normalized = value.trim().toLowerCase(Locale.ROOT);
        for (ClientVariant variant : values()) {
            if (variant.externalName.equals(normalized)) {
                return variant;
            }
        }
        throw new IllegalArgumentException("unknown JAVA_HTTP_CLIENT_VARIANT: " + value);
    }

    String externalName() {
        return externalName;
    }

    String completionStrategy() {
        return completionStrategy;
    }

    String executor() {
        return executor;
    }

    static void configureSystemProperties(ClientVariant variant) {
        if (variant == SYNC_TUNED) {
            System.setProperty("jdk.httpclient.connectionPoolSize", "100");
            System.setProperty("jdk.httpclient.keepalive.timeout", "30");
        }
    }
}
