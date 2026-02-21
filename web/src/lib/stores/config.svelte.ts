import { type Settings } from "../api-types";

class ConfigStore {
    #settings = $state<Settings | null>(null);
    #initialized = $state(false);

    setSettings(s: Settings) {
        this.#settings = s;
        this.#initialized = true;
    }

    get settings() {
        return this.#settings;
    }

    get initialized() {
        return this.#initialized;
    }

    // AI is considered active if enabled AND at least one test has passed for the current config
    get aiActive() {
        if (!this.#settings) return false;
        return this.#settings.aiEnabled && this.#settings.aiTestingPassed;
    }

    // Portainer is active if enabled AND connectivity test passed
    get portainerActive() {
        if (!this.#settings) return false;
        return this.#settings.portainerEnabled && this.#settings.portainerTestingPassed;
    }
}

export const configStore = new ConfigStore();
