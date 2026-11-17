package mock

import (
	"encoding/json/v2"
	"fmt"
	"maps"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// defaultScenario is the scenario of a route nobody set (rule 2).
const defaultScenario = "default"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetScenario selects the scenario a route answers with (rule 2). An empty
// name, or "default", goes back to the default. An unknown route or
// scenario is core.ErrInvalid. Safe for concurrent use.
func (t *Transport) SetScenario(routeID, name string) error {
	if err := t.check(routeID, name); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if name == "" || name == defaultScenario {
		delete(t.scenarios, routeID)

		return nil
	}
	t.scenarios[routeID] = name

	return nil
}

// Scenarios returns a copy of the scenarios set: route id to name.
func (t *Transport) Scenarios() map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return maps.Clone(t.scenarios)
}

// LoadScenarios replaces the scenarios with those of a JSON object of route
// id to scenario name, the format of local/mock-scenarios.json and of the
// "scenarios" of prefs.json. It changes nothing when one entry is invalid.
func (t *Transport) LoadScenarios(data []byte) error {
	var in map[string]string
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("mock: scenarios: %w: %w", core.ErrInvalid, err)
	}
	next := make(map[string]string, len(in))
	for id, name := range in {
		if err := t.check(id, name); err != nil {
			return err
		}
		if name != "" && name != defaultScenario {
			next[id] = name
		}
	}
	t.mu.Lock()
	t.scenarios = next
	t.mu.Unlock()

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// check reports an unknown route or scenario.
func (t *Transport) check(routeID, name string) error {
	r := t.route(routeID)
	if r == nil {
		return fmt.Errorf("mock: route %q: %w", routeID, core.ErrInvalid)
	}
	if name == "" {
		return nil
	}
	if _, ok := r.Responses[name]; !ok {
		return fmt.Errorf("mock: scenario %q of route %q: %w", name, routeID, core.ErrInvalid)
	}

	return nil
}

// scenarioOf returns the scenario a route answers with now.
func (t *Transport) scenarioOf(routeID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if n, ok := t.scenarios[routeID]; ok {
		return n
	}

	return defaultScenario
}
