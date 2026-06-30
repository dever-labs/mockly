package io.mockly.driver.model;

import java.util.Map;

/**
 * A single HTTP request recorded by the Mockly server.
 */
public class CallEntry {
    private final String id;
    private final String timestamp;
    private final String protocol;
    private final String method;
    private final String path;
    private final int status;
    private final long durationMs;
    private final Map<String, String> headers;
    private final String body;
    private final String matchedId;
    private final Map<String, String> pathParams;

    public CallEntry(
            String id,
            String timestamp,
            String protocol,
            String method,
            String path,
            int status,
            long durationMs,
            Map<String, String> headers,
            String body,
            String matchedId,
            Map<String, String> pathParams) {
        this.id = id;
        this.timestamp = timestamp;
        this.protocol = protocol;
        this.method = method;
        this.path = path;
        this.status = status;
        this.durationMs = durationMs;
        this.headers = headers;
        this.body = body;
        this.matchedId = matchedId;
        this.pathParams = pathParams;
    }

    public String getId()                    { return id; }
    public String getTimestamp()             { return timestamp; }
    public String getProtocol()             { return protocol; }
    public String getMethod()               { return method; }
    public String getPath()                 { return path; }
    public int    getStatus()               { return status; }
    public long   getDurationMs()           { return durationMs; }
    public Map<String, String> getHeaders() { return headers; }
    public String getBody()                 { return body; }
    public String getMatchedId()            { return matchedId; }
    public Map<String, String> getPathParams() { return pathParams; }
}
