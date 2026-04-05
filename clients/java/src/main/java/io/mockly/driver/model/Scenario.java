package io.mockly.driver.model;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/** A named scenario that patches one or more mock responses when activated. */
public class Scenario {
    private final String id;
    private final String name;
    private final List<ScenarioPatch> patches;

    private Scenario(Builder builder) {
        this.id = builder.id;
        this.name = builder.name;
        this.patches = Collections.unmodifiableList(new ArrayList<>(builder.patches));
    }

    public String getId() { return id; }
    public String getName() { return name; }
    public List<ScenarioPatch> getPatches() { return patches; }

    public static Builder builder(String id, String name) {
        return new Builder(id, name);
    }

    public static class Builder {
        private final String id;
        private final String name;
        private final List<ScenarioPatch> patches = new ArrayList<>();

        private Builder(String id, String name) {
            this.id = id;
            this.name = name;
        }

        public Builder patch(ScenarioPatch patch) {
            patches.add(patch);
            return this;
        }

        public Builder patches(List<ScenarioPatch> patches) {
            this.patches.addAll(patches);
            return this;
        }

        public Scenario build() {
            return new Scenario(this);
        }
    }
}
