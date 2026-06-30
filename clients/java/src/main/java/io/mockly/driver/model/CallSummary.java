package io.mockly.driver.model;

import java.util.List;
import java.util.Map;

/**
 * Summary of recorded calls for a specific HTTP mock.
 */
public class CallSummary {
    private final String mockId;
    private final long count;
    private final List<CallEntry> calls;

    public CallSummary(String mockId, long count, List<CallEntry> calls) {
        this.mockId = mockId;
        this.count = count;
        this.calls = calls;
    }

    /** The mock ID these calls belong to. */
    public String getMockId() { return mockId; }

    /** Total number of times this mock was called. */
    public long getCount() { return count; }

    /** The individual recorded request entries. */
    public List<CallEntry> getCalls() { return calls; }
}
