<script lang="ts">
    import { onMount } from "svelte";
    import type { ContainerSummary, Metric } from "../api-types";
    import Sparkline from "../components/Sparkline.svelte";
    import { parseImageRef } from "../utils/image-ref";

    let { containers, onNavigate } = $props<{
        containers: ContainerSummary[];
        onNavigate: (route: string, params?: any) => void;
    }>();

    let sparklineMetrics = $state<Record<string, Metric[]>>({});
    let lastSparklineKey = $state("");
    let intelByContainer = $state<Record<string, ContainerIntelReadiness>>({});
    let lastIntelKey = $state("");
    let imageRiskByKey = $state<Record<string, { critical: number; high: number; malwareInfected: boolean }>>({});
    let lastRiskKey = $state("");
    let searchQuery = $state("");
    type FleetFilter = "all" | "updates" | "intel" | "high-risk" | "ignored";
    let activeFilter = $state<FleetFilter>("all");
    let ignoredTokens = $state<string[]>(["harborwatch"]);
    let loadingIgnoreTokens = $state(false);
    let loadingIntelReadiness = $state(false);

    interface ContainerIntelIssue {
        code: string;
        severity: "info" | "warning" | "error";
        message: string;
        action?: string;
    }

    interface ContainerIntelReadiness {
        containerId: string;
        effectiveRepositoryUrl?: string;
        effectiveChangelogUrl?: string;
        repositoryProvider?: string;
        hasRepository?: boolean;
        hasChangelog?: boolean;
        releaseIntelReady?: boolean;
        fullAutomationReady?: boolean;
        issues?: ContainerIntelIssue[];
    }

    const formatId = (id: string) => (id.length > 12 ? id.slice(0, 12) : id);

    const stateColor = (state: string) => {
        switch (state.toLowerCase()) {
            case "running":
                return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400";
            case "exited":
                return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400";
            case "paused":
                return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
            default:
                return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-400";
        }
    };

    const healthColor = (health: string) => {
        switch (health.toLowerCase()) {
            case "healthy":
                return "bg-emerald-500";
            case "unhealthy":
                return "bg-rose-500";
            case "starting":
                return "bg-amber-500";
            default:
                return "";
        }
    };

    const getPolicy = (labels: Record<string, string>) => {
        if (!labels) return null;
        return labels["harborwatch.update.policy"] || (labels["harborwatch.enable"] === "true" ? "auto" : null);
    };

    const getIntelURL = (labels: Record<string, string>) => {
        if (!labels) return null;
        return labels["harborwatch.intel.url"] || labels["org.opencontainers.image.source"] || labels["org.label-schema.vcs-url"];
    };

    function lookupIntel(summary: ContainerSummary): ContainerIntelReadiness | null {
        const id = String(summary.id || "").trim();
        if (!id) return null;
        if (intelByContainer[id]) return intelByContainer[id];
        for (const [key, value] of Object.entries(intelByContainer)) {
            if (!key) continue;
            if (id.startsWith(key) || key.startsWith(id)) return value;
        }
        return null;
    }

    function effectiveRepoURL(summary: ContainerSummary): string {
        const intel = lookupIntel(summary);
        const effective = String(intel?.effectiveRepositoryUrl || "").trim();
        if (effective) return effective;
        return String(getIntelURL(summary.labels) || "").trim();
    }

    function intelNeedsAttention(summary: ContainerSummary): boolean {
        const intel = lookupIntel(summary);
        if (!intel) return false;
        return intel.fullAutomationReady === false || (intel.issues || []).length > 0;
    }

    function intelPrimaryIssue(summary: ContainerSummary): string {
        const intel = lookupIntel(summary);
        if (!intel) return "";
        const first = (intel.issues || [])[0];
        return String(first?.message || "").trim();
    }

    function normalizeImageKey(raw: string): string {
        let value = String(raw || "").trim().toLowerCase();
        if (!value) return "";
        const at = value.indexOf("@");
        if (at > 0) value = value.slice(0, at);
        value = value.replace(/^docker\.io\//, "");
        value = value.replace(/^index\.docker\.io\//, "");
        value = value.replace(/^registry-1\.docker\.io\//, "");
        value = value.replace(/^library\//, "");
        return value;
    }

    function imageRisk(summary: ContainerSummary): { critical: number; high: number; malwareInfected: boolean } {
        const key = normalizeImageKey(summary.image);
        return imageRiskByKey[key] || { critical: 0, high: 0, malwareInfected: false };
    }

    function isHighRisk(summary: ContainerSummary): boolean {
        const risk = imageRisk(summary);
        return risk.malwareInfected || risk.critical > 0 || risk.high > 0;
    }

    function handleTriggerScan(image: string) {
        onNavigate("security", { target: image });
    }

    const imageRepo = (image: string) => parseImageRef(image).repository;
    const imageQualifier = (image: string) => parseImageRef(image).qualifier || ":latest";

    let safeContainers = $derived(containers || []);
    const normalizedSearch = $derived(searchQuery.trim().toLowerCase());

    function splitDelimitedTokens(raw: string): string[] {
        return String(raw || "")
            .split(/[,\n;\t\r]+/g)
            .map((token) => token.trim().toLowerCase())
            .filter((token) => token.length > 0);
    }

    function containerSearchText(summary: ContainerSummary): string {
        const values: string[] = [];
        values.push(String(summary.id || ""));
        values.push(String(summary.image || ""));
        values.push(String(summary.state || ""));
        for (const name of summary.names || []) values.push(String(name || "").replace(/^\//, ""));
        const labels = summary.labels || {};
        for (const [key, value] of Object.entries(labels)) {
            values.push(`${key}:${String(value || "")}`);
        }
        return values.join(" ").toLowerCase();
    }

    function matchesIgnoredToken(summary: ContainerSummary, token: string): boolean {
        const normalized = String(token || "").trim().toLowerCase();
        if (!normalized) return false;

        const id = String(summary.id || "").trim().toLowerCase();
        if (id && (id === normalized || id.startsWith(normalized))) return true;

        const image = String(summary.image || "").trim().toLowerCase();
        if (image && (image === normalized || image.includes(normalized))) return true;

        for (const name of summary.names || []) {
            const normalizedName = String(name || "").trim().replace(/^\//, "").toLowerCase();
            if (!normalizedName) continue;
            if (normalizedName === normalized || normalizedName.includes(normalized)) return true;
        }

        const labels = summary.labels || {};
        for (const value of Object.values(labels)) {
            const labelValue = String(value || "").trim().toLowerCase();
            if (!labelValue) continue;
            if (labelValue === normalized || labelValue.includes(normalized)) return true;
        }
        return false;
    }

    function isAutomationIgnored(summary: ContainerSummary): boolean {
        return ignoredTokens.some((token) => matchesIgnoredToken(summary, token));
    }

    let filteredContainers = $derived(
        safeContainers.filter((summary) => {
            if (!normalizedSearch) return true;
            return containerSearchText(summary).includes(normalizedSearch);
        })
    );

    let visibleContainers = $derived(
        filteredContainers.filter((summary) => {
            switch (activeFilter) {
                case "updates":
                    return !!summary.updateAvailable;
                case "intel":
                    return intelNeedsAttention(summary);
                case "high-risk":
                    return isHighRisk(summary);
                case "ignored":
                    return isAutomationIgnored(summary);
                default:
                    return true;
            }
        })
    );

    function latestMetric(id: string): Metric | null {
        const series = sparklineMetrics[id] || [];
        return series.length > 0 ? series[series.length - 1] : null;
    }

    function formatPercent(value: number | undefined): string {
        const n = Number(value || 0);
        if (!Number.isFinite(n)) return "0%";
        return `${n.toFixed(1)}%`;
    }

    function formatBytes(value: number | undefined): string {
        let n = Number(value || 0);
        if (!Number.isFinite(n) || n <= 0) return "0 B";
        const units = ["B", "KB", "MB", "GB", "TB"];
        let idx = 0;
        while (n >= 1024 && idx < units.length - 1) {
            n /= 1024;
            idx++;
        }
        return `${n.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
    }

    function memoryRatio(metric: Metric | null): number | null {
        if (!metric || !metric.memoryLimit || metric.memoryLimit <= 0) return null;
        const ratio = (metric.memoryUsage / metric.memoryLimit) * 100;
        return Math.max(0, Math.min(100, ratio));
    }

    async function loadSparklineMetrics(ids: string[]) {
        if (ids.length === 0) {
            sparklineMetrics = {};
            return;
        }
        try {
            const res = await fetch("/api/metrics/batch", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ ids, duration: "1h" })
            });
            if (!res.ok) return;
            const data = await res.json();
            sparklineMetrics = data || {};
        } catch (e) {
            console.error("Failed to fetch sparkline metrics batch", e);
        }
    }

    async function loadIgnoredContainerTokens() {
        loadingIgnoreTokens = true;
        try {
            const res = await fetch("/api/settings");
            if (!res.ok) {
                ignoredTokens = ["harborwatch"];
                return;
            }
            const payload = await res.json();
            const merged = splitDelimitedTokens(String(payload?.automationIgnoredContainers || ""));
            if (!merged.includes("harborwatch")) merged.push("harborwatch");
            ignoredTokens = Array.from(new Set(merged));
        } catch {
            ignoredTokens = ["harborwatch"];
        } finally {
            loadingIgnoreTokens = false;
        }
    }

    async function loadIntelReadiness() {
        loadingIntelReadiness = true;
        try {
            const res = await fetch("/api/docker/containers/intel-readiness");
            if (!res.ok) {
                intelByContainer = {};
                return;
            }
            const rows = await res.json();
            const next: Record<string, ContainerIntelReadiness> = {};
            if (Array.isArray(rows)) {
                for (const row of rows) {
                    const key = String(row?.containerId || "").trim();
                    if (!key) continue;
                    next[key] = row as ContainerIntelReadiness;
                }
            }
            intelByContainer = next;
        } catch {
            intelByContainer = {};
        } finally {
            loadingIntelReadiness = false;
        }
    }

    async function loadImageRiskSignals() {
        try {
            const res = await fetch("/api/docker/images/intelligence");
            if (!res.ok) {
                imageRiskByKey = {};
                return;
            }
            const rows = await res.json();
            const next: Record<string, { critical: number; high: number; malwareInfected: boolean }> = {};
            if (Array.isArray(rows)) {
                for (const row of rows) {
                    const candidates = [row?.primaryRef, ...(Array.isArray(row?.repoTags) ? row.repoTags : [])];
                    for (const candidate of candidates) {
                        const key = normalizeImageKey(String(candidate || ""));
                        if (!key) continue;
                        next[key] = {
                            critical: Number(row?.vulnerabilityCritical || 0),
                            high: Number(row?.vulnerabilityHigh || 0),
                            malwareInfected: !!row?.malwareInfected
                        };
                    }
                }
            }
            imageRiskByKey = next;
        } catch {
            imageRiskByKey = {};
        }
    }

    $effect(() => {
        const ids = safeContainers.map((c) => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastSparklineKey) return;
        lastSparklineKey = key;
        loadSparklineMetrics(ids);
    });

    $effect(() => {
        const ids = safeContainers.map((c) => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastIntelKey) return;
        lastIntelKey = key;
        loadIntelReadiness();
    });

    $effect(() => {
        const refs = safeContainers.map((c) => normalizeImageKey(c.image)).filter(Boolean);
        const key = refs.join(",");
        if (key === lastRiskKey) return;
        lastRiskKey = key;
        loadImageRiskSignals();
    });

    onMount(() => {
        loadIgnoredContainerTokens();
    });
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Fleet Inventory</h2>
            <p class="text-xs text-slate-500 font-medium">Card-first status view with quick drill-down into container control pages.</p>
        </div>
        <div class="flex items-center gap-2 flex-wrap justify-end">
            <div class="flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-2.5 py-1.5">
                <input
                    bind:value={searchQuery}
                    placeholder="Search fleet..."
                    class="w-44 md:w-56 bg-transparent text-[11px] text-slate-700 dark:text-slate-200 outline-none"
                />
                {#if searchQuery.trim()}
                    <button
                        type="button"
                        onclick={() => (searchQuery = "")}
                        class="text-[10px] font-black uppercase tracking-widest text-slate-500 hover:text-brand-600"
                    >
                        Clear
                    </button>
                {/if}
            </div>
            <span class="px-3 py-1 bg-brand-50 dark:bg-brand-900/30 rounded-full text-[10px] font-black text-brand-700 dark:text-brand-300 uppercase tracking-widest border border-brand-200 dark:border-brand-700/50">
                Card View
            </span>
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                {visibleContainers.length}/{safeContainers.length} Visible
            </span>
        </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
        {#each [
            { id: "all", label: "All" },
            { id: "updates", label: "Upgrade Needed" },
            { id: "intel", label: "Intel Issues" },
            { id: "high-risk", label: "High Risk" },
            { id: "ignored", label: "Ignored" }
        ] as filter}
            <button
                type="button"
                onclick={() => activeFilter = filter.id as FleetFilter}
                class="px-3 py-1.5 rounded-full border text-[10px] font-black uppercase tracking-widest transition-colors {activeFilter === filter.id ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-300'}"
            >
                {filter.label}
            </button>
        {/each}
    </div>

    {#if loadingIgnoreTokens}
        <p class="text-[11px] text-slate-500">Resolving automation ignore state...</p>
    {/if}

    <div class="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-5">
        {#each visibleContainers as c, i}
            {@const current = latestMetric(c.id)}
            {@const memoryPct = memoryRatio(current)}
            <article
                class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden flex flex-col group hover:border-brand-500 transition-all opacity-0 animate-reveal"
                style="animation-delay: {0.08 + (i * 0.03)}s"
            >
                <div class="p-5 flex-1 space-y-4">
                    <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                            <button
                                onclick={() => onNavigate("container-detail", { id: c.id })}
                                class="font-black text-slate-900 dark:text-white truncate text-lg tracking-tight hover:text-brand-600 transition-colors text-left"
                                title="Open container details"
                            >
                                {c.names?.[0]?.replace(/^\//, "") ?? "unnamed"}
                            </button>
                            <p class="text-[10px] font-mono text-slate-400 mt-1 uppercase tracking-widest">{formatId(c.id)}</p>
                        </div>
                        <div class="flex items-center gap-2">
                            {#if c.updateAvailable}
                                <span class="px-2 py-1 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded-lg text-[9px] font-black uppercase tracking-widest">
                                    Update
                                </span>
                            {/if}
                            {#if intelNeedsAttention(c)}
                                <span class="px-2 py-1 bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 rounded-lg text-[9px] font-black uppercase tracking-widest" title={intelPrimaryIssue(c) || "Container intelligence requires attention"}>
                                    Intel
                                </span>
                            {/if}
                            {#if isAutomationIgnored(c)}
                                <span class="px-2 py-1 bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200 rounded-lg text-[9px] font-black uppercase tracking-widest">
                                    Ignored
                                </span>
                            {/if}
                            {#if c.health && c.health !== "none"}
                                <div class="flex items-center gap-1.5 px-2 py-1 bg-slate-100 dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
                                    <span class="w-1.5 h-1.5 rounded-full {healthColor(c.health)} animate-pulse"></span>
                                    <span class="text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-400">{c.health}</span>
                                </div>
                            {/if}
                            <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase {stateColor(c.state)}">
                                {c.state}
                            </span>
                        </div>
                    </div>

                    <div class="space-y-1">
                        <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest">Image</p>
                        <div class="flex items-center gap-2 min-w-0" title={c.image}>
                            <span class="text-xs text-slate-600 dark:text-slate-300 truncate font-bold">{imageRepo(c.image)}</span>
                            <span class="px-1.5 py-0.5 rounded-md bg-slate-100 dark:bg-slate-700 text-[9px] font-black uppercase tracking-tight text-slate-600 dark:text-slate-300 whitespace-nowrap">{imageQualifier(c.image)}</span>
                        </div>
                    </div>

                    <!-- Simplified Metrics Section -->
                    <div class="py-2 space-y-3">
                        <div class="flex items-center justify-between">
                            <div class="flex flex-col">
                                <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">Resource Load</span>
                                <div class="flex items-baseline gap-1.5">
                                    <span class="text-xl font-black text-slate-900 dark:text-white">{formatPercent(current?.cpuPercent)}</span>
                                    <span class="text-[10px] font-bold text-slate-400 uppercase">CPU</span>
                                </div>
                            </div>
                            <div class="w-32 h-10">
                                <Sparkline metrics={sparklineMetrics[c.id] || []} />
                            </div>
                        </div>

                        <div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-slate-700/50">
                            <div class="flex flex-col">
                                <span class="text-[9px] font-black uppercase tracking-widest text-slate-400">Memory</span>
                                <span class="text-xs font-bold text-slate-700 dark:text-slate-200">{formatBytes(current?.memoryUsage)}</span>
                            </div>
                            <div class="flex flex-col items-end">
                                <span class="text-[9px] font-black uppercase tracking-widest text-slate-400">Threads</span>
                                <span class="text-xs font-bold text-slate-700 dark:text-slate-200">{current?.pids ?? 0} PIDs</span>
                            </div>
                        </div>
                    </div>

                    <div class="flex items-center justify-between text-[10px] pt-1">
                        <span class="text-slate-500 uppercase tracking-widest font-black">Automation Policy</span>
                        {#if getPolicy(c.labels)}
                            <span class="px-2 py-0.5 bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-400 rounded-md text-[9px] font-black uppercase tracking-tighter">
                                {getPolicy(c.labels)}
                            </span>
                        {:else}
                            <span class="text-slate-400 italic font-medium">Standard (Manual)</span>
                        {/if}
                    </div>

                    {#if intelNeedsAttention(c)}
                        <div class="rounded-lg border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-900/10 px-2.5 py-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-rose-700 dark:text-rose-300">AI Automation Needs Metadata</p>
                            <p class="mt-1 text-[10px] text-rose-700/90 dark:text-rose-200/90">{intelPrimaryIssue(c) || "Open Manage > Intelligence and configure repository/changelog overrides."}</p>
                        </div>
                    {/if}
                </div>

                <div class="px-5 py-3 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700 flex justify-between items-center">
                    <div class="flex gap-1">
                        {#if effectiveRepoURL(c)}
                            <a
                                href={effectiveRepoURL(c)}
                                target="_blank"
                                class="p-2 text-slate-400 hover:text-brand-600 transition-colors"
                                title="Open source repository"
                            >
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                                </svg>
                            </a>
                        {/if}
                    </div>
                    <div class="flex gap-2">
                        <button
                            onclick={() => handleTriggerScan(c.image)}
                            class="px-3 py-2 text-[10px] font-black uppercase tracking-widest text-slate-500 hover:text-brand-600 transition-all"
                        >
                            Scan
                        </button>
                        <button
                            onclick={() => onNavigate("container-detail", { id: c.id })}
                            class="px-4 py-2 bg-brand-600 text-white rounded-xl text-[10px] font-black uppercase tracking-widest shadow-lg shadow-brand-500/20 hover:bg-brand-700 transition-all"
                        >
                            Manage
                        </button>
                    </div>
                </div>
            </article>
        {:else}
            <div class="col-span-full py-12 text-center text-slate-400 italic bg-slate-50 dark:bg-slate-900/50 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-800">
                {#if normalizedSearch}
                    No containers matched your search
                {:else if activeFilter !== "all"}
                    No containers matched the selected filter
                {:else}
                    No containers found on socket
                {/if}
            </div>
        {/each}
    </div>

    {#if loadingIntelReadiness}
        <p class="text-[11px] text-slate-500">Resolving container intelligence readiness...</p>
    {/if}
</div>
