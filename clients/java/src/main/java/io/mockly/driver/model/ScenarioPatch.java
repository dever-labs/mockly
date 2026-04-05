package io.mockly.driver.model;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/** Represents a partial update to a scenario (e.g. add/remove mocks). */
public class ScenarioPatch {
    private final List<Mock> addMocks;
    private final List<String> removeMockIds;

    private ScenarioPatch(Builder builder) {
        this.addMocks = Collections.unmodifiableList(new ArrayList<>(builder.addMocks));
        this.removeMockIds = Collections.unmodifiableList(new ArrayList<>(builder.removeMockIds));
    }

    public List<Mock> getAddMocks() { return addMocks; }
    public List<String> getRemoveMockIds() { return removeMockIds; }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private final List<Mock> addMocks = new ArrayList<>();
        private final List<String> removeMockIds = new ArrayList<>();

        public Builder addMock(Mock mock) {
            addMocks.add(mock);
            return this;
        }

        public Builder removeMockId(String id) {
            removeMockIds.add(id);
            return this;
        }

        public ScenarioPatch build() {
            return new ScenarioPatch(this);
        }
    }
}
