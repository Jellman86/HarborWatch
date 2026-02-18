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
    let selectedPreset = $state<PresetID>("all");
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

    function logsEndpoint(): string {
        const params = new URLSearchParams({ limit: "50" });
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
        const preset = normalizePreset(params?.preset);
        applyPreset(preset, false);
        loadData();
        const interval = setInterval(loadData, 5000);
        return () => clearInterval(interval);
    });

    $effect(() => {
        const preset = normalizePreset(params?.preset);
        if (preset !== selectedPreset) {
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

    let visibleLogs = $derived(logs.filter((entry) => matchesPreset(entry, selectedPreset)));

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
                return "bg-emerald-950/50 text-emerald-300 border border-emerald-800/60";
            case "automation":
                return "bg-blue-950/50 text-blue-300 border border-blue-800/60";
            case "updates":
                return "bg-violet-950/50 text-violet-300 border border-violet-800/60";
            case "errors":
                return "bg-rose-950/50 text-rose-300 border border-rose-800/60";
            case "audit":
                return "bg-amber-950/50 text-amber-300 border border-amber-800/60";
            default:
                return "bg-slate-800 text-slate-300 border border-slate-700";
        }
    }

    function applyLogSearch() {
        loading = true;
        void loadData();
    }

    function applyPreset(id: PresetID, reload = true) {
        selectedPreset = id;
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
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">System Health</h2>
            <p class="text-xs text-slate-500 font-medium">Real-time appliance telemetry and internal execution logs.</p>
        </div>
        <div class="flex items-center gap-2">
            <div class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
            <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">Live Telemetry</span>
        </div>
    </div>

    {#if status}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 opacity-0 animate-reveal stagger-1">
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Uptime</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatUptime(status.uptime)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Memory (Alloc)</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatBytes(status.memoryAlloc)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Database Size</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatBytes(status.dbSize)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Active Routines</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{status.numGoroutine}</p>
            </div>
        </div>
    {/if}

    <div class="space-y-4 opacity-0 animate-reveal stagger-2">
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <h3 class="text-xs font-black uppercase text-slate-400 tracking-widest flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                Internal Application Logs
            </h3>
            <div class="flex items-center gap-2 flex-wrap">
                {#each presets as preset}
                    <button
                        type="button"
                        onclick={() => applyPreset(preset.id)}
                        aria-pressed={selectedPreset === preset.id}
                        class="px-3 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all {(selectedPreset === preset.id) ? 'bg-brand-600 text-white ring-2 ring-brand-300/70 scale-[1.02]' : 'bg-slate-800/80 border border-slate-700 text-slate-300 hover:bg-slate-700'}"
                    >
                        {preset.label}
                    </button>
                {/each}
            </div>
        </div>
        <div class="text-[11px] text-slate-500">
            Audit Trail now maps to the <span class="font-bold text-slate-300">System Health</span> stream via the <span class="font-bold text-brand-400">Audit Trail</span> preset.
        </div>
        <div class="flex items-center justify-between rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2 text-[11px]">
            <div class="flex items-center gap-2">
                <span class="px-2 py-1 rounded-md bg-brand-600/30 text-brand-300 font-black uppercase tracking-widest">{activePresetLabel}</span>
                <span class="text-slate-400">Showing {visibleLogs.length} of {logs.length} log entries</span>
            </div>
            {#if loading}
                <span class="text-slate-400 font-black uppercase tracking-widest animate-pulse">Refreshing...</span>
            {/if}
        </div>
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <form
                class="flex items-center gap-2"
                onsubmit={(e) => {
                    e.preventDefault();
                    applyLogSearch();
                }}
            >
                <input
                    bind:value={logSearch}
                    placeholder="Search logs..."
                    class="w-56 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                />
                <select
                    bind:value={logLevel}
                    class="bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                >
                    <option value="">All Levels</option>
                    <option value="ERROR">ERROR</option>
                    <option value="WARN">WARN</option>
                    <option value="INFO">INFO</option>
                </select>
                <select
                    bind:value={logSource}
                    class="bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-200 outline-none focus:ring-2 focus:ring-brand-500"
                >
                    <option value="">All Sources</option>
                    <option value="scanner">Scanner</option>
                    <option value="scheduler">Scheduler</option>
                    <option value="updateengine">UpdateEngine</option>
                    <option value="docker">Docker</option>
                    <option value="system">System</option>
                </select>
                <button type="submit" class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 text-white text-[10px] font-black uppercase tracking-widest">Search</button>
                <button type="button" onclick={() => applyPreset("all")} class="px-3 py-2 rounded-xl border border-slate-700 text-slate-300 text-[10px] font-black uppercase tracking-widest hover:bg-slate-800/60">Clear</button>
            </form>
        </div>
        
        <div class="bg-slate-900 rounded-3xl border border-slate-800 shadow-2xl overflow-hidden">
            <div class="overflow-x-auto">
                <table class="w-full text-left border-collapse">
                    <thead>
                        <tr class="bg-slate-950/50 text-[10px] font-black uppercase tracking-widest text-slate-500 border-b border-slate-800">
                            <th class="px-6 py-3 w-48">Timestamp</th>
                            <th class="px-6 py-3 w-24">Level</th>
                            <th class="px-6 py-3 w-28">Class</th>
                            <th class="px-6 py-3 w-32">Source</th>
                            <th class="px-6 py-3">Message</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800/50 font-mono text-[11px]">
                        {#each visibleLogs as log}
                            <tr class="hover:bg-slate-800/30 transition-colors">
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
                                <td class="px-6 py-2 text-slate-300">
                                    {log.message}
                                </td>
                            </tr>
                        {:else}
                            <tr>
                                <td colspan="5" class="px-6 py-12 text-center text-slate-600 italic">No logs matched the current preset/filters.</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>
