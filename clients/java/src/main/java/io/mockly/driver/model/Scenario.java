package io.mockly.driver.model;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public class Scenario {
    private final String id;
    private final String description;
    private final List<Mock> mocks;

    private Scenario(Builder builder) {
        this.id = builder.id;
        this.description = builder.description;
        this.mocks = Collections.unmodifiableList(new ArrayList<>(builder.mocks));
    }

    public String getId() { return id; }
    public String getDescription() { return description; }
    public List<Mock> getMocks() { return mocks; }

    public static Builder builder(String id) {
        return new Builder(id);
    }

    public static class Builder {
        private final String id;
        private String description;
        private final List<Mock> mocks = new ArrayList<>();

        private Builder(String id) {
            this.id = id;
        }

        public Builder description(String description) {
            this.description = description;
            return this;
        }

        public Builder mock(Mock mock) {
            mocks.add(mock);
            return this;
        }

        public Builder mocks(List<Mock> mocks) {
            this.mocks.addAll(mocks);
            return this;
        }

        public Scenario build() {
            return new Scenario(this);
        }
    }
}
