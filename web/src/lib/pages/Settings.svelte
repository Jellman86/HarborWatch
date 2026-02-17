<script lang="ts">
    import { onMount } from "svelte";
    import AutomationFlowChart from "../components/AutomationFlowChart.svelte";
    import ThemeSwitcher from "../components/ThemeSwitcher.svelte";
    import type { Settings } from "../api-types";
    import { toasts } from "../stores/ToastStore";

    interface Schedule {
        id: string;
        cronSpec: string;
        enabled: boolean;
        lastRun?: number;
    }

    interface ModelList {
        models?: string[];
        error?: string;
    }

    interface ModelCatalog {
        providers?: Record<string, ModelList>;
    }

    type AutomationDomain = "upgrades" | "maintenance" | "security";

    const defaultSettings: Settings = {
        discordWebhookUrl: "",
        gotifyUrl: "",
        gotifyToken: "",
        portainerUrl: "",
        portainerApiKey: "",
        aiProvider: "",
        openaiKey: "",
        openaiModel: "",
        anthropicKey: "",
        anthropicModel: "",
        geminiKey: "",
        geminiModel: "",
        instanceUrl: "",
        validateUrlPattern: "",
        environmentOverrides: {}
    };

    let settings = $state<Settings>({ ...defaultSettings });
    let schedules = $state<Schedule[]>([]);
    let modelCatalog = $state<ModelCatalog>({ providers: {} });

    let activeTab = $state("automations");
    let activeAutomationTab = $state<AutomationDomain>("upgrades");

    let loading = $state(false);
    let saving = $state(false);
    let loadingModels = $state(false);
    let testingProvider = $state("");

    const automationConfig: Record<AutomationDomain, {
        title: string;
        subtitle: string;
        accent: string;
        tasks: string[];
        flow: string[];
    }> = {
        upgrades: {
            title: "Upgrade Automation",
            subtitle: "Detect updates, assess risk, and prepare safe rollouts",
            accent: "#0ea5e9",
            tasks: ["container_update_check"],
            flow: ["Discover Tags", "Risk Review", "Stage Update", "Health Verify", "Promote/Rollback"]
        },
        maintenance: {
            title: "Maintenance Automation",
            subtitle: "Keep host resources healthy and control data growth",
            accent: "#14b8a6",
            tasks: ["docker_system_prune", "metrics_prune", "diag_log_prune"],
            flow: ["Measure Usage", "Prune Targets", "Reclaim Space", "Verify Capacity", "Notify Team"]
        },
        security: {
            title: "Security Automation",
            subtitle: "Continuously sweep vulnerabilities and malware",
            accent: "#f97316",
            tasks: ["security_sweep_trivy", "malware_sweep_clamav"],
            flow: ["Inventory Assets", "Run Trivy", "Run ClamAV", "Prioritize Findings", "Escalate Action"]
        }
    };

    function isLocked(key: string) {
        return settings.environmentOverrides?.[key] || false;
    }

    function scheduleById(id: string): Schedule | undefined {
        return schedules.find((s) => s.id === id);
    }

    function schedulesForDomain(domain: AutomationDomain): Schedule[] {
        return automationConfig[domain].tasks
            .map((id) => scheduleById(id))
            .filter((v): v is Schedule => !!v);
    }

    function domainEnabled(domain: AutomationDomain): boolean {
        const scoped = schedulesForDomain(domain);
        return scoped.length > 0 && scoped.some((s) => s.enabled);
    }

    function flowSteps(domain: AutomationDomain): Array<{ label: string; state: "active" | "idle" | "warning" }> {
        const scoped = schedulesForDomain(domain);
        const total = Math.max(scoped.length, 1);
        const active = scoped.filter((s) => s.enabled).length;
        return automationConfig[domain].flow.map((label, idx, arr) => {
            if (active === 0) return { label, state: "idle" as const };
            if (active >= total) return { label, state: "active" as const };
            if (idx === 0 || idx === arr.length - 1) return { label, state: "active" as const };
            return { label, state: "warning" as const };
        });
    }

    async function loadSettings() {
        const res = await fetch("/api/settings");
        if (!res.ok) throw new Error(`settings read failed (${res.status})`);
        const data = await res.json();
        data.environmentOverrides = data.environmentOverrides || {};
        settings = { ...defaultSettings, ...data };
    }

    async function loadSchedules() {
        const res = await fetch("/api/scheduler/schedules");
        if (!res.ok) throw new Error(`schedules read failed (${res.status})`);
        schedules = await res.json();
    }

    async function loadModelCatalog() {
        loadingModels = true;
        try {
            const res = await fetch("/api/ai/models");
            if (!res.ok) throw new Error(`model catalog failed (${res.status})`);
            modelCatalog = await res.json();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to load AI models");
        } finally {
            loadingModels = false;
        }
    }

    async function loadAll() {
        loading = true;
        try {
            await Promise.all([loadSettings(), loadSchedules(), loadModelCatalog()]);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to load settings");
        } finally {
            loading = false;
        }
    }

    async function saveSettings() {
        saving = true;
        try {
            const res = await fetch("/api/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(settings)
            });
            if (!res.ok) throw new Error(`settings save failed (${res.status})`);
            toasts.success("Settings saved.");
            await loadSettings();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to save settings");
        } finally {
            saving = false;
        }
    }

    async function toggleTask(id: string, enabled: boolean) {
        try {
            const res = await fetch("/api/scheduler/toggle", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ id, enabled: !enabled })
            });
            if (!res.ok) throw new Error(`toggle failed (${res.status})`);
            schedules = schedules.map((s) => (s.id === id ? { ...s, enabled: !enabled } : s));
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to toggle automation task");
        }
    }

    async function runTask(id: string) {
        try {
            const res = await fetch("/api/scheduler/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ id })
            });
            if (!res.ok) throw new Error(`run failed (${res.status})`);
            toasts.success(`Task ${taskLabel(id)} triggered.`);
            await loadSchedules();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to run task");
        }
    }

    function providerModels(provider: "openai" | "anthropic" | "gemini"): string[] {
        return modelCatalog.providers?.[provider]?.models || [];
    }

    function providerError(provider: "openai" | "anthropic" | "gemini"): string {
        return modelCatalog.providers?.[provider]?.error || "";
    }

    async function testProvider(provider: "openai" | "anthropic" | "gemini", model: string) {
        testingProvider = provider;
        try {
            const res = await fetch("/api/ai/test", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ provider, model })
            });
            const body = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(body?.message || `AI test failed (${res.status})`);
            toasts.success(`AI test passed: ${provider} (${model || "default"})`);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "AI test failed");
        } finally {
            testingProvider = "";
        }
    }

    function cronLabel(spec: string): string {
        if (spec === "0 0 3 * * 0") return "Weekly (Sun 03:00)";
        if (spec === "0 0 * * * *") return "Hourly";
        if (spec === "0 * * * * *") return "Every Minute";
        if (spec === "0 0 0 * * *") return "Daily (00:00)";
        if (spec === "0 0 4 * * 0") return "Weekly (Sun 04:00)";
        if (spec === "0 0 1 * * *") return "Daily (01:00)";
        return spec;
    }

    function taskLabel(id: string): string {
        switch (id) {
            case "container_update_check": return "Container Update Check";
            case "docker_system_prune": return "Docker System Prune";
            case "metrics_prune": return "Metrics Retention Prune";
            case "diag_log_prune": return "Diagnostics Log Prune";
            case "security_sweep_trivy": return "Trivy Security Sweep";
            case "malware_sweep_clamav": return "ClamAV Malware Sweep";
            default: return id.replace(/_/g, " ");
        }
    }

    function formatTime(ts?: number): string {
        return ts && ts > 0 ? new Date(ts * 1000).toLocaleString() : "Never";
    }

    onMount(() => {
        loadAll();
    });
</script>

<div class="w-full space-y-8">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tight">Settings</h2>
            <p class="text-sm text-slate-500 mt-1">Global configuration and automation policy control plane.</p>
        </div>
        <button
            onclick={saveSettings}
            disabled={saving || loading}
            class="px-6 py-3 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20"
        >
            {saving ? "Saving..." : "Save Settings"}
        </button>
    </div>

    <div class="flex flex-wrap gap-2 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-800 w-fit">
        {#each [
            { id: "automations", label: "Automations" },
            { id: "ai", label: "AI" },
            { id: "integrations", label: "Integrations" },
            { id: "system", label: "System" },
            { id: "appearance", label: "Appearance" }
        ] as tab}
            <button
                onclick={() => activeTab = tab.id}
                class="px-5 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
            >
                {tab.label}
            </button>
        {/each}
    </div>

    <div class="w-full bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm min-h-[620px]">
        {#if loading}
            <div class="p-10 text-sm text-slate-500">Loading settings...</div>
        {:else if activeTab === "automations"}
            <div class="p-6 md:p-8 space-y-6">
                <div class="flex flex-wrap gap-2 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-700 w-fit">
                    {#each [
                        { id: "upgrades", label: "Upgrades" },
                        { id: "maintenance", label: "Maintenance" },
                        { id: "security", label: "Security" }
                    ] as tab}
                        <button
                            onclick={() => activeAutomationTab = tab.id as AutomationDomain}
                            class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeAutomationTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                        >
                            {tab.label}
                        </button>
                    {/each}
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-slate-50/60 dark:bg-slate-900/40">
                        <AutomationFlowChart
                            title={automationConfig[activeAutomationTab].title}
                            subtitle={automationConfig[activeAutomationTab].subtitle}
                            accent={automationConfig[activeAutomationTab].accent}
                            steps={flowSteps(activeAutomationTab)}
                        />
                        <p class="mt-2 text-xs text-slate-500">
                            Global policy is managed by scheduler tasks. Containers can inherit this policy or override it in container lifecycle settings.
                        </p>
                    </div>

                    <div class="space-y-3">
                        {#each schedulesForDomain(activeAutomationTab) as task}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-white dark:bg-slate-900/30">
                                <div class="flex flex-wrap items-center justify-between gap-3">
                                    <div>
                                        <p class="text-sm font-black text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                                        <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold">{cronLabel(task.cronSpec)} | Last run: {formatTime(task.lastRun)}</p>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <button
                                            onclick={() => runTask(task.id)}
                                            class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                                        >Run Now</button>
                                        <button
                                            onclick={() => toggleTask(task.id, task.enabled)}
                                            class="w-10 h-5 rounded-full relative transition-colors {task.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                            aria-label="Toggle task"
                                        >
                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {task.enabled ? 'right-1' : 'left-1'}"></div>
                                        </button>
                                    </div>
                                </div>
                            </div>
                        {:else}
                            <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 p-6 text-sm text-slate-500 italic">
                                No scheduler tasks found for this automation domain.
                            </div>
                        {/each}
                    </div>
                </div>

                <div class="rounded-2xl border border-brand-200 dark:border-brand-900/40 bg-brand-50 dark:bg-brand-900/10 p-4 text-xs text-brand-800 dark:text-brand-300">
                    Domain status: <strong>{domainEnabled(activeAutomationTab) ? "Enabled" : "Disabled"}</strong>
                    : If disabled, inherited container policies in this domain do not run.
                </div>
            </div>

        {:else if activeTab === "ai"}
            <div class="p-6 md:p-8 space-y-8">
                <div class="flex flex-wrap items-end gap-4">
                    <div class="space-y-2 min-w-[260px]">
                        <label for="ai-provider" class="text-[10px] font-black uppercase text-slate-400 ml-1">Preferred Provider</label>
                        <select id="ai-provider" bind:value={settings.aiProvider} disabled={isLocked("aiProvider")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            <option value="">Auto (first configured)</option>
                            <option value="openai">OpenAI</option>
                            <option value="anthropic">Anthropic</option>
                            <option value="gemini">Gemini</option>
                        </select>
                    </div>
                    <button onclick={loadModelCatalog} disabled={loadingModels} class="px-4 py-3 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50">
                        {loadingModels ? "Refreshing..." : "Refresh Model Catalog"}
                    </button>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">OpenAI</h3>
                        <input type="password" bind:value={settings.openaiKey} disabled={isLocked("openaiKey")} placeholder="sk-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {#if providerModels("openai").length > 0}
                            <select bind:value={settings.openaiModel} disabled={isLocked("openaiModel")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                                <option value="">Default</option>
                                {#each providerModels("openai") as model}
                                    <option value={model}>{model}</option>
                                {/each}
                            </select>
                        {:else}
                            <input bind:value={settings.openaiModel} disabled={isLocked("openaiModel")} placeholder="gpt-5-mini" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {/if}
                        {#if providerError("openai")}
                            <p class="text-[11px] text-amber-600">{providerError("openai")}</p>
                        {/if}
                        <button onclick={() => testProvider("openai", settings.openaiModel || "")} disabled={testingProvider === "openai"} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "openai" ? "Testing..." : "Test OpenAI"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Anthropic</h3>
                        <input type="password" bind:value={settings.anthropicKey} disabled={isLocked("anthropicKey")} placeholder="sk-ant-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {#if providerModels("anthropic").length > 0}
                            <select bind:value={settings.anthropicModel} disabled={isLocked("anthropicModel")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                                <option value="">Default</option>
                                {#each providerModels("anthropic") as model}
                                    <option value={model}>{model}</option>
                                {/each}
                            </select>
                        {:else}
                            <input bind:value={settings.anthropicModel} disabled={isLocked("anthropicModel")} placeholder="claude-sonnet" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {/if}
                        {#if providerError("anthropic")}
                            <p class="text-[11px] text-amber-600">{providerError("anthropic")}</p>
                        {/if}
                        <button onclick={() => testProvider("anthropic", settings.anthropicModel || "")} disabled={testingProvider === "anthropic"} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "anthropic" ? "Testing..." : "Test Anthropic"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Gemini</h3>
                        <input type="password" bind:value={settings.geminiKey} disabled={isLocked("geminiKey")} placeholder="AIza..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {#if providerModels("gemini").length > 0}
                            <select bind:value={settings.geminiModel} disabled={isLocked("geminiModel")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                                <option value="">Default</option>
                                {#each providerModels("gemini") as model}
                                    <option value={model}>{model}</option>
                                {/each}
                            </select>
                        {:else}
                            <input bind:value={settings.geminiModel} disabled={isLocked("geminiModel")} placeholder="gemini-2.5-flash" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        {/if}
                        {#if providerError("gemini")}
                            <p class="text-[11px] text-amber-600">{providerError("gemini")}</p>
                        {/if}
                        <button onclick={() => testProvider("gemini", settings.geminiModel || "")} disabled={testingProvider === "gemini"} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "gemini" ? "Testing..." : "Test Gemini"}</button>
                    </div>
                </div>
            </div>

        {:else if activeTab === "integrations"}
            <div class="p-6 md:p-8 space-y-8">
                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
                    <div class="space-y-2">
                        <label for="discord-webhook" class="text-[10px] font-black uppercase text-slate-400 ml-1">Discord Webhook</label>
                        <input id="discord-webhook" type="password" bind:value={settings.discordWebhookUrl} disabled={isLocked("discordWebhookUrl")} placeholder="https://discord.com/api/webhooks/..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2">
                        <label for="gotify-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Gotify URL</label>
                        <input id="gotify-url" bind:value={settings.gotifyUrl} disabled={isLocked("gotifyUrl")} placeholder="https://gotify.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2">
                        <label for="gotify-token" class="text-[10px] font-black uppercase text-slate-400 ml-1">Gotify Token</label>
                        <input id="gotify-token" type="password" bind:value={settings.gotifyToken} disabled={isLocked("gotifyToken")} placeholder="A..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2">
                        <label for="portainer-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer URL</label>
                        <input id="portainer-url" bind:value={settings.portainerUrl} disabled={isLocked("portainerUrl")} placeholder="https://portainer.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2 xl:col-span-2">
                        <label for="portainer-api-key" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer API Key</label>
                        <input id="portainer-api-key" type="password" bind:value={settings.portainerApiKey} disabled={isLocked("portainerApiKey")} placeholder="ptr_..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                </div>
            </div>

        {:else if activeTab === "system"}
            <div class="p-6 md:p-8 grid grid-cols-1 xl:grid-cols-2 gap-6">
                <div class="space-y-2">
                    <label for="instance-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Instance URL</label>
                    <input id="instance-url" bind:value={settings.instanceUrl} disabled={isLocked("instanceUrl")} placeholder="https://harborwatch.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    <p class="text-[11px] text-slate-500">Used for webhook callbacks and external links.</p>
                </div>
                <div class="space-y-2">
                    <label for="validate-pattern" class="text-[10px] font-black uppercase text-slate-400 ml-1">Validation URL Pattern</label>
                    <input id="validate-pattern" bind:value={settings.validateUrlPattern} disabled={isLocked("validateUrlPattern")} placeholder={"http://localhost:{{PORT}}/health"} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    <p class="text-[11px] text-slate-500">Template for deriving per-container validation URLs.</p>
                </div>
            </div>

        {:else if activeTab === "appearance"}
            <div class="p-6 md:p-8">
                <ThemeSwitcher />
            </div>
        {/if}

        <div class="px-6 md:px-8 py-4 border-t border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 text-[11px] text-slate-500">
            Some fields may be read-only when defined via environment variables.
        </div>
    </div>
</div>
