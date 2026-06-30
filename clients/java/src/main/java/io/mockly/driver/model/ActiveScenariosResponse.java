package io.mockly.driver.model;

import java.util.Collections;
import java.util.List;

/** Active scenario IDs together with their full definitions. */
public class ActiveScenariosResponse {
    private final List<String> active;
    private final List<Scenario> scenarios;

    public ActiveScenariosResponse(List<String> active, List<Scenario> scenarios) {
        this.active = Collections.unmodifiableList(active);
        this.scenarios = Collections.unmodifiableList(scenarios);
    }

    public List<String> getActive() { return active; }
    public List<Scenario> getScenarios() { return scenarios; }
}
