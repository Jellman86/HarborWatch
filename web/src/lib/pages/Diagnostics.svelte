<script lang="ts">
    import { onMount } from "svelte";

    interface SystemStatus {
        uptime: number;
        numGoroutine: number;
        memoryAlloc: number;
        dbSize: number;
    }

    interface LogEntry {
        timestamp: number;
        level: string;
        source: string;
        message: string;
    }

    type PresetID = "all" | "audit" | "security" | "automation" | "updates" | "errors";
    interface LogPreset {
        id: PresetID;
        label: string;
    }

    let { params } = $props<{
        params?: { preset?: string };
    }>();

    const presets: LogPreset[] = [
        { id: "all", label: "All Logs" },
        { id: "audit", label: "Audit Trail" },
        { id: "security", label: "Security" },
        { id: "automation", label: "Automation" },
        { id: "updates", label: "Updates" },
        { id: "errors", label: "Errors" }
    ];

    let status = $state<SystemStatus | null>(null);
    let logs = $state<LogEntry[]>([]);
    let loading = $state(true);
    let logSearch = $state("");
    let logLevel = $state("");
    let logSource = $state("");
    let logPage = $state(0);
    let logLimit = 100;
    let selectedPreset = $state<PresetID>("all");
    let hideMetricsCollectorNoise = $state(true);
    let activePresetLabel = $derived(presets.find((p) => p.id === selectedPreset)?.label || "All Logs");

    function normalizePreset(raw?: string): PresetID {
        const value = String(raw || "").trim().toLowerCase();
        switch (value) {
            case "audit":
            case "security":
            case "automation":
            case "updates":
            case "errors":
                return value as PresetID;
            default:
                return "all";
        }
    }

    function routePreset(raw?: string): PresetID | null {
        const trimmed = String(raw || "").trim();
        if (!trimmed) {
            return null;
        }
        return normalizePreset(trimmed);
    }

    function logsEndpoint(): string {
        const offset = logPage * logLimit;
        const params = new URLSearchParams({ 
            limit: String(logLimit),
            offset: String(offset)
        });
        const query = String(logSearch || "").trim();
        const level = String(logLevel || "").trim();
        const source = String(logSource || "").trim();
        if (query) params.set("search", query);
        if (level) params.set("level", level);
        if (source) params.set("source", source);
        return `/api/system/logs?${params.toString()}`;
    }

    async function loadData() {
        try {
            const [statusRes, logsRes] = await Promise.all([
                fetch("/api/system/status"),
                fetch(logsEndpoint())
            ]);
            if (statusRes.ok) status = await statusRes.json();
            if (logsRes.ok) logs = await logsRes.json();
        } catch (e) {
            console.error("Failed to load diagnostics", e);
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        const preset = routePreset(params?.preset);
        if (preset) {
            applyPreset(preset, false);
        }
        loadData();
        const interval = setInterval(loadData, 5000);
        return () => clearInterval(interval);
    });

    $effect(() => {
        const preset = routePreset(params?.preset);
        if (preset && preset !== selectedPreset) {
            applyPreset(preset);
        }
    });

    const formatBytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(2) + " MB";
    const formatUptime = (seconds: number) => {
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return `${h}h ${m}m`;
    };

    const levelColor = (level: string) => {
        switch(level.toLowerCase()) {
            case 'error': return 'text-rose-500';
            case 'warn': return 'text-amber-500';
            default: return 'text-slate-400';
        }
    };

    function classifyLog(entry: LogEntry): "audit" | "security" | "automation" | "updates" | "errors" | "all" {
        const source = String(entry.source || "").toLowerCase();
        const message = String(entry.message || "").toLowerCase();
        const level = String(entry.level || "").toLowerCase();
        if (level === "error") return "errors";
        if (source.includes("updateengine") || message.includes("update pipeline") || message.includes("rollback")) return "updates";
        if (source.includes("scheduler") || message.includes("task ") || message.includes("prune")) return "automation";
        if (source.includes("scanner") || message.includes("trivy") || message.includes("clamav") || message.includes("malware")) return "security";
        if (source.includes("scanner") || source.includes("scheduler") || source.includes("updateengine") || message.includes("job")) return "audit";
        return "all";
    }

    function matchesPreset(entry: LogEntry, preset: PresetID): boolean {
        if (preset === "all") return true;
        const cls = classifyLog(entry);
        if (preset === "audit") {
            return cls === "audit" || cls === "security" || cls === "automation" || cls === "updates" || cls === "errors";
        }
        return cls === preset;
    }

    function isMetricsCollectorNoise(entry: LogEntry): boolean {
        const source = String(entry.source || "").toLowerCase();
        if (!source.includes("scheduler")) return false;
        const message = String(entry.message || "").toLowerCase();
        return message.includes("metrics_collector") &&
            (message.includes("executing scheduled task") || message.includes("scheduled task completed"));
    }

    let visibleLogs = $derived(
        logs.filter((entry) => matchesPreset(entry, selectedPreset))
            .filter((entry) => !hideMetricsCollectorNoise || !isMetricsCollectorNoise(entry))
    );

    function classBadge(log: LogEntry): string {
        const cls = classifyLog(log);
        switch (cls) {
            case "security":
                return "Security";
            case "automation":
                return "Automation";
            case "updates":
                return "Updates";
            case "errors":
                return "Error";
            case "audit":
                return "Audit";
            default:
                return "General";
        }
    }

    function classColor(log: LogEntry): string {
        const cls = classifyLog(log);
        switch (cls) {
            case "security":
                return "bg-emerald-100 text-emerald-700 border border-emerald-200 dark:bg-emerald-950/50 dark:text-emerald-300 dark:border-emerald-800/60";
            case "automation":
                return "bg-blue-100 text-blue-700 border border-blue-200 dark:bg-blue-950/50 dark:text-blue-300 dark:border-blue-800/60";
            case "updates":
                return "bg-violet-100 text-violet-700 border border-violet-200 dark:bg-violet-950/50 dark:text-violet-300 dark:border-violet-800/60";
            case "errors":
                return "bg-rose-100 text-rose-700 border border-rose-200 dark:bg-rose-950/50 dark:text-rose-300 dark:border-rose-800/60";
            case "audit":
                return "bg-amber-100 text-amber-700 border border-amber-200 dark:bg-amber-950/50 dark:text-amber-300 dark:border-amber-800/60";
            default:
                return "bg-slate-100 text-slate-700 border border-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700";
        }
    }

    function applyLogSearch() {
        logPage = 0;
        loading = true;
        void loadData();
    }

    function applyPreset(id: PresetID, reload = true) {
        selectedPreset = id;
        logPage = 0;
        if (id === "errors") {
            logLevel = "ERROR";
        } else if (logLevel === "ERROR") {
            logLevel = "";
        }
        if (id === "security") {
            logSource = "scanner";
        } else if (id === "automation") {
            logSource = "scheduler";
        } else if (id === "updates") {
            logSource = "updateengine";
        } else if (id === "all" || id === "audit") {
            logSource = "";
        }
        if (reload) {
            applyLogSearch();
        }
    }

    function changePage(delta: number) {
        logPage = Math.max(0, logPage + delta);
        loading = true;
        void loadData();
    }
</script>

<div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">System Health</h2>
            <p class="text-xs text-slate-500 font-medium">Internal execution logs and appliance telemetry.</p>
        </div>
        {#if status}
            <div class="flex flex-wrap items-center gap-3">
                <div class="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 shadow-sm">
                    <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Uptime</span>
                    <span class="text-xs font-bold text-slate-700 dark:text-slate-200">{formatUptime(status.uptime)}</span>
                </div>
                <div class="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 shadow-sm">
                    <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Mem</span>
                    <span class="text-xs font-bold text-slate-700 dark:text-slate-200">{formatBytes(status.memoryAlloc)}</span>
                </div>
                <div class="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 shadow-sm">
                    <div class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></div>
                    <span class="text-[9px] font-black uppercase tracking-widest text-slate-400">Live</span>
                </div>
            </div>
        {/if}
    </div>

    <div class="space-y-4 opacity-0 animate-reveal stagger-1">
        <div class="flex flex-col gap-4">
            <div class="flex flex-wrap items-center gap-2">
                {#each presets as preset}
                    <button
                        type="button"
                        onclick={() => applyPreset(preset.id)}
                        aria-pressed={selectedPreset === preset.id}
                        class="flex-1 md:flex-none px-3 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {(selectedPreset === preset.id) ? 'bg-brand-600 text-white ring-2 ring-brand-300/70 scale-[1.02]' : 'bg-white dark:bg-slate-900/40 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800'}"
                    >
                        {preset.label}
                    </button>
                {/each}
            </div>
        </div>
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white/80 dark:bg-slate-900/40 px-3 py-2 text-[11px]">
            <div class="flex items-center justify-between md:justify-start w-full md:w-auto gap-2">
                <span class="px-2 py-1 rounded-md bg-brand-600/30 text-brand-300 font-black uppercase tracking-widest">{activePresetLabel}</span>
                <span class="text-slate-500 dark:text-slate-400">Showing {visibleLogs.length} entries</span>
            </div>
            {#if loading}
                <span class="text-slate-500 dark:text-slate-400 font-black uppercase tracking-widest animate-pulse">Refreshing...</span>
            {/if}
        </div>
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <form
                class="flex flex-wrap items-center gap-2"
                onsubmit={(e) => {
                    e.preventDefault();
                    applyLogSearch();
                }}
            >
                <input
                    bind:value={logSearch}
                    placeholder="Search logs..."
                    class="flex-1 md:w-56 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-700 dark:text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                />
                <select
                    bind:value={logLevel}
                    class="w-[calc(50%-0.25rem)] md:w-auto bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-700 dark:text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                >
                    <option value="">All Levels</option>
                    <option value="ERROR">ERROR</option>
                    <option value="WARN">WARN</option>
                    <option value="INFO">INFO</option>
                </select>
                <select
                    bind:value={logSource}
                    class="w-[calc(50%-0.25rem)] md:w-auto bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-700 dark:text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                >
                    <option value="">All Sources</option>
                    <option value="scanner">Scanner</option>
                    <option value="scheduler">Scheduler</option>
                    <option value="updateengine">UpdateEngine</option>
                    <option value="docker">Docker</option>
                    <option value="system">System</option>
                </select>
                <div class="flex items-center gap-2 w-full md:w-auto">
                    <button type="submit" class="flex-1 md:flex-none px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 text-white text-[10px] font-black uppercase tracking-widest">Search</button>
                    <button type="button" onclick={() => applyPreset("all")} class="flex-1 md:flex-none px-3 py-2 rounded-xl border border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800/60">Clear</button>
                </div>
            </form>
            <button
                type="button"
                onclick={() => (hideMetricsCollectorNoise = !hideMetricsCollectorNoise)}
                class="w-full md:w-auto px-3 py-2 rounded-xl border text-[10px] font-black uppercase tracking-widest transition-colors {hideMetricsCollectorNoise ? 'border-brand-200 text-brand-700 bg-brand-50 dark:bg-brand-900/30 dark:text-brand-300 dark:border-brand-800/60' : 'border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800/60'}"
            >
                {hideMetricsCollectorNoise ? "Hide Metrics Noise" : "Show Metrics Noise"}
            </button>
        </div>

        <div class="flex items-center justify-between gap-4 py-2 px-1">
            <div class="flex items-center gap-2">
                <button
                    onclick={() => changePage(-1)}
                    disabled={logPage === 0 || loading}
                    class="px-4 py-2 rounded-xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-700 disabled:opacity-50 transition-all"
                >
                    Previous
                </button>
                <button
                    onclick={() => changePage(1)}
                    disabled={logs.length < logLimit || loading}
                    class="px-4 py-2 rounded-xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-700 disabled:opacity-50 transition-all"
                >
                    Next
                </button>
            </div>
            <div class="text-[10px] font-black text-slate-400 uppercase tracking-widest">
                Page {logPage + 1}
            </div>
        </div>

        <div class="md:hidden space-y-2">
            {#each visibleLogs as log, i (i)}
                <article class="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-700 p-3 space-y-1.5">
                    <div class="flex items-center justify-between gap-2">
                        <p class="text-[10px] font-mono text-slate-500">{new Date(log.timestamp * 1000).toISOString().replace('T', ' ').split('.')[0]}</p>
                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {classColor(log)}">{classBadge(log)}</span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="text-[10px] font-black uppercase {levelColor(log.level)}">{log.level}</span>
                        <span class="text-[10px] text-brand-600 dark:text-brand-300 font-bold">{log.source}</span>
                    </div>
                    <p class="text-[11px] text-slate-700 dark:text-slate-300">{log.message}</p>
                </article>
            {:else}
                <div class="px-4 py-8 text-center text-slate-500 italic text-sm bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-700">
                    No logs matched the current preset/filters.
                </div>
            {/each}
        </div>

        <div class="hidden md:block bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden">
            <div class="overflow-x-auto">
                <table class="w-full text-left border-collapse">
                    <thead>
                        <tr class="bg-slate-100 dark:bg-slate-950/50 text-[10px] font-black uppercase tracking-widest text-slate-500 border-b border-slate-200 dark:border-slate-800">
                            <th class="px-6 py-3 w-48">Timestamp</th>
                            <th class="px-6 py-3 w-24">Level</th>
                            <th class="px-6 py-3 w-28">Class</th>
                            <th class="px-6 py-3 w-32">Source</th>
                            <th class="px-6 py-3">Message</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-200 dark:divide-slate-800/50 font-mono text-[11px]">
                        {#each visibleLogs as log, i (i)}
                            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                                <td class="px-6 py-2 text-slate-500">
                                    {new Date(log.timestamp * 1000).toISOString().replace('T', ' ').split('.')[0]}
                                </td>
                                <td class="px-6 py-2 font-black uppercase {levelColor(log.level)}">
                                    {log.level}
                                </td>
                                <td class="px-6 py-2">
                                    <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {classColor(log)}">
                                        {classBadge(log)}
                                    </span>
                                </td>
                                <td class="px-6 py-2 text-brand-500 font-bold">
                                    {log.source}
                                </td>
                                <td class="px-6 py-2 text-slate-700 dark:text-slate-300">
                                    {log.message}
                                </td>
                            </tr>
                        {:else}
                            <tr><td colspan="5" class="px-6 py-12 text-center text-slate-500 italic">No logs matched the current preset/filters.</td></tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>
