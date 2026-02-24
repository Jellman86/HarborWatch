<script lang="ts">
    import { onMount } from "svelte";
    import type { ContainerSummary, Metric } from "../api-types";
    import Sparkline from "../components/Sparkline.svelte";
    import PortainerLogo from "../components/PortainerLogo.svelte";
    import { parseImageRef } from "../utils/image-ref";
    import { configStore } from "../stores/config.svelte";

    let { containers, params, onNavigate } = $props<{
        containers: ContainerSummary[];
        params?: { search?: string };
        onNavigate: (route: string, params?: any) => void;
    }>();

    let sparklineMetrics = $state<Record<string, Metric[]>>({});
    let lastSparklineKey = $state("");
    let intelByContainer = $state<Record<string, ContainerIntelReadiness>>({});
    let lastIntelKey = $state("");
    let imageRiskByKey = $state<Record<string, { critical: number; high: number; malwareInfected: boolean }>>({});
    let lastRiskKey = $state("");
    let aiBlockedByContainer = $state<Record<string, AIBlockedSignal>>({});
    let lastAIBlockedKey = $state("");
    let searchQuery = $state("");
    let showIgnored = $state(false);
    let sortBy = $state<"name" | "state" | "memory" | "cpu">("name");
    let pageIndex = $state(0);
    const pageSize = 24;

    $effect(() => {
        if (params?.search) {
            searchQuery = params.search;
            // Auto-show ignored if searching for something specific
            if (params.search.trim()) showIgnored = true;
        }
        if (params?.filter) {
            activeFilter = params.filter as FleetFilter;
        }
    });

    $effect(() => {
        if (params?.search) {
            searchQuery = params.search;
        }
    });

    type FleetFilter = "all" | "updates" | "intel" | "high-risk" | "ignored";
    let activeFilter = $state<FleetFilter>("all");
    let ignoredTokens = $state<string[]>(["harborwatch", "portainer", "portainer-ce", "ix-portainer"]);
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
        portainerManaged?: boolean;
        portainerConfigured?: boolean;
        issues?: ContainerIntelIssue[];
    }

    interface AIBlockedSignal {
        blocked: boolean;
        reason?: string;
        riskScore?: number;
        riskLevel?: string;
        updatedAt?: number;
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

    function lookupAIBlocked(summary: ContainerSummary): AIBlockedSignal | null {
        const id = String(summary.id || "").trim();
        if (!id) return null;
        if (aiBlockedByContainer[id]) return aiBlockedByContainer[id];
        for (const [key, value] of Object.entries(aiBlockedByContainer)) {
            if (!key) continue;
            if (id.startsWith(key) || key.startsWith(id)) return value;
        }
        return null;
    }

    function hasAIBlocked(summary: ContainerSummary): boolean {
        return !!lookupAIBlocked(summary)?.blocked;
    }

    function aiBlockedTooltip(summary: ContainerSummary): string {
        const signal = lookupAIBlocked(summary);
        if (!signal?.blocked) return "";
        const parts: string[] = [];
        if (signal.riskLevel) parts.push(`Risk ${signal.riskLevel}${signal.riskScore ? ` (${signal.riskScore})` : ""}`);
        if (signal.reason) parts.push(signal.reason);
        if (signal.updatedAt) parts.push(`Last blocked ${new Date(signal.updatedAt * 1000).toLocaleString()}`);
        return parts.join(" • ");
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

    function handleTriggerScan(id: string) {
        onNavigate("container-detail", { id, tab: "security" });
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
        if (id && id.includes(normalized)) return true;

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
        safeContainers.filter((summary: ContainerSummary) => {
            if (!normalizedSearch) return true;
            return containerSearchText(summary).includes(normalizedSearch);
        })
    );

    let visibleContainers = $derived(
        filteredContainers.filter((summary: ContainerSummary) => {
            const ignored = isAutomationIgnored(summary);
            // If searching explicitly, always show matches regardless of ignore state
            if (normalizedSearch) return true;
            // Otherwise, filter by ignore status if toggle is off
            if (!showIgnored && ignored && activeFilter !== "ignored") return false;

            switch (activeFilter) {
                case "updates":
                    return !!summary.updateAvailable;
                case "intel":
                    return intelNeedsAttention(summary);
                case "high-risk":
                    return isHighRisk(summary);
                case "ignored":
                    return ignored;
                default:
                    return true;
            }
        }).sort((a: ContainerSummary, b: ContainerSummary) => {
            if (sortBy === "name") {
                const nameA = (a.names?.[0] || "").toLowerCase();
                const nameB = (b.names?.[0] || "").toLowerCase();
                return nameA.localeCompare(nameB);
            }
            if (sortBy === "state") {
                return a.state.localeCompare(b.state);
            }
            if (sortBy === "memory") {
                const metA = latestMetric(a.id);
                const metB = latestMetric(b.id);
                return (metB?.memoryUsage || 0) - (metA?.memoryUsage || 0);
            }
            if (sortBy === "cpu") {
                const metA = latestMetric(a.id);
                const metB = latestMetric(b.id);
                return (metB?.cpuPercent || 0) - (metA?.cpuPercent || 0);
            }
            return 0;
        })
    );

    let ignoredCount = $derived(
        filteredContainers.filter((c: ContainerSummary) => isAutomationIgnored(c)).length
    );

    let hiddenIgnoredCount = $derived(
        !showIgnored ? filteredContainers.filter((c: ContainerSummary) => isAutomationIgnored(c) && activeFilter !== "ignored").length : 0
    );

    let totalContainerPages = $derived(Math.max(1, Math.ceil(visibleContainers.length / pageSize)));
    let pagedVisibleContainers = $derived(
        visibleContainers.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize)
    );
    let visiblePageStart = $derived(visibleContainers.length === 0 ? 0 : (pageIndex * pageSize) + 1);
    let visiblePageEnd = $derived(Math.min((pageIndex + 1) * pageSize, visibleContainers.length));

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
                ignoredTokens = ["harborwatch", "portainer", "portainer-ce", "ix-portainer"];
                return;
            }
            const payload = await res.json();
            const merged = splitDelimitedTokens(String(payload?.automationIgnoredContainers || ""));
            const defaults = ["harborwatch", "portainer", "portainer-ce", "ix-portainer"];
            for (const d of defaults) {
                if (!merged.includes(d)) merged.push(d);
            }
            ignoredTokens = Array.from(new Set(merged));
        } catch {
            ignoredTokens = ["harborwatch", "portainer", "portainer-ce", "ix-portainer"];
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

    async function loadAIBlockedSignals() {
        try {
            const res = await fetch("/api/updates/ai-blocked");
            if (!res.ok) {
                aiBlockedByContainer = {};
                return;
            }
            const payload = await res.json();
            const next: Record<string, AIBlockedSignal> = {};
            if (payload && typeof payload === "object") {
                for (const [id, value] of Object.entries(payload)) {
                    const key = String(id || "").trim();
                    if (!key || !value || typeof value !== "object") continue;
                    next[key] = value as AIBlockedSignal;
                }
            }
            aiBlockedByContainer = next;
        } catch {
            aiBlockedByContainer = {};
        }
    }

    $effect(() => {
        const ids = safeContainers.map((c: ContainerSummary) => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastSparklineKey) return;
        lastSparklineKey = key;
        loadSparklineMetrics(ids);
    });

    $effect(() => {
        const ids = safeContainers.map((c: ContainerSummary) => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastIntelKey) return;
        lastIntelKey = key;
        loadIntelReadiness();
    });

    $effect(() => {
        const refs = safeContainers.map((c: ContainerSummary) => normalizeImageKey(c.image)).filter(Boolean);
        const key = refs.join(",");
        if (key === lastRiskKey) return;
        lastRiskKey = key;
        loadImageRiskSignals();
    });

    $effect(() => {
        const ids = safeContainers.map((c: ContainerSummary) => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastAIBlockedKey) return;
        lastAIBlockedKey = key;
        loadAIBlockedSignals();
    });

    $effect(() => {
        normalizedSearch;
        activeFilter;
        showIgnored;
        sortBy;
        safeContainers.length;
        pageIndex = 0;
    });

    $effect(() => {
        if (pageIndex > totalContainerPages - 1) {
            pageIndex = Math.max(0, totalContainerPages - 1);
        }
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
                {#if hiddenIgnoredCount > 0}
                    <span class="ml-1 text-slate-400">({hiddenIgnoredCount} Hidden)</span>
                {/if}
            </span>
            <span class="px-3 py-1 bg-white dark:bg-slate-900/40 rounded-full text-[10px] font-black text-slate-500 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                Page {pageIndex + 1}/{totalContainerPages}
            </span>
        </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-4">
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

        <div class="flex items-center gap-2">
            <select
                bind:value={sortBy}
                class="bg-white dark:bg-slate-900/40 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-1.5 text-[10px] font-black uppercase tracking-widest outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-600 dark:text-slate-300"
            >
                <option value="name">Sort: Name</option>
                <option value="state">Sort: State</option>
                <option value="memory">Sort: Memory</option>
                <option value="cpu">Sort: CPU</option>
            </select>

            <button
                type="button"
                onclick={() => showIgnored = !showIgnored}
                class="flex items-center gap-2 px-3 py-1.5 rounded-xl border text-[10px] font-black uppercase tracking-widest transition-all {showIgnored ? 'bg-brand-50 border-brand-200 text-brand-700 dark:bg-brand-900/20 dark:border-brand-800' : 'bg-white border-slate-200 text-slate-500 dark:bg-slate-900/40 dark:border-slate-700'}"
            >
                <div class="w-3.5 h-3.5 rounded border flex items-center justify-center transition-colors {showIgnored ? 'bg-brand-600 border-brand-600 text-white' : 'border-slate-300 dark:border-slate-600 text-transparent'}">
                    {#if showIgnored}
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
                            <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                        </svg>
                    {/if}
                </div>
                Show Ignored
            </button>
        </div>
    </div>

    {#if visibleContainers.length > 0}
        <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 px-4 py-3">
            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">
                Showing {visiblePageStart}-{visiblePageEnd} of {visibleContainers.length}
            </p>
            <div class="flex items-center gap-2">
                <button
                    type="button"
                    onclick={() => pageIndex = Math.max(0, pageIndex - 1)}
                    disabled={pageIndex === 0}
                    class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 disabled:opacity-50 hover:border-brand-300"
                >
                    Prev
                </button>
                <button
                    type="button"
                    onclick={() => pageIndex = Math.min(totalContainerPages - 1, pageIndex + 1)}
                    disabled={pageIndex >= totalContainerPages - 1}
                    class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 disabled:opacity-50 hover:border-brand-300"
                >
                    Next
                </button>
            </div>
        </div>
    {/if}

    {#if loadingIgnoreTokens}
        <p class="text-[11px] text-slate-500">Resolving automation ignore state...</p>
    {/if}

    <div class="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-5">
        {#each pagedVisibleContainers as c, i}
            {@const current = latestMetric(c.id)}
            {@const memoryPct = memoryRatio(current)}
            {@const ignored = isAutomationIgnored(c)}
            <article
                class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden flex flex-col group hover:border-brand-500 transition-all opacity-0 animate-reveal {ignored ? 'opacity-60 grayscale-[0.8] brightness-95' : ''}"
                style="animation-delay: {0.08 + (i * 0.03)}s"
            >
                <div class="p-5 flex-1 space-y-4">
                    <div class="flex flex-wrap items-start justify-between gap-3 min-w-0">
                        <div class="min-w-0 flex-1">
                            <button
                                onclick={() => onNavigate("container-detail", { id: c.id })}
                                class="font-black text-slate-900 dark:text-white truncate text-lg tracking-tight hover:text-brand-600 transition-colors text-left w-full"
                                title="Open container details"
                            >
                                {c.names?.[0]?.replace(/^\//, "") ?? "unnamed"}
                            </button>
                            <p class="text-[10px] font-mono text-slate-400 mt-1 uppercase tracking-widest">{formatId(c.id)}</p>
                        </div>
                        <div class="flex flex-wrap items-center justify-end gap-1.5 flex-shrink-0">
                            {#if c.updateAvailable}
                                <span class="p-1.5 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded-lg" title="Software Update Available">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                                        <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd" />
                                    </svg>
                                </span>
                            {/if}
                            {#if configStore.portainerActive && lookupIntel(c)?.portainerManaged}
                                <span class="p-1.5 {lookupIntel(c)?.portainerConfigured ? 'bg-cyan-500/10 text-cyan-500' : 'bg-amber-500 text-white'} rounded-lg" title={lookupIntel(c)?.portainerConfigured ? "Managed by Portainer" : "Portainer integration required for safe updates"}>
                                    <PortainerLogo size={14} />
                                </span>
                            {/if}
                            {#if intelNeedsAttention(c)}
                                <span class="p-1.5 bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 rounded-lg" title={intelPrimaryIssue(c) || "Container intelligence requires attention"}>
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                                    </svg>
                                </span>
                            {/if}
                            {#if hasAIBlocked(c)}
                                <span class="px-2 py-1 bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 rounded-lg text-[9px] font-black uppercase tracking-tight border border-rose-200 dark:border-rose-900/40" title={aiBlockedTooltip(c)}>
                                    AI Blocked
                                </span>
                            {/if}
                            {#if ignored}
                                <span class="p-1.5 bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200 rounded-lg" title="Ignored from Automation">
                                    <!-- Ghost icon for ignored/stealth -->
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                                        <path d="M10 2a6 6 0 00-6 6v3.586l-.707.707A1 1 0 004 14h12a1 1 0 00.707-1.707L16 11.586V8a6 6 0 00-6-6zM10 18a3 3 0 01-3-3h6a3 3 0 01-3 3z" />
                                    </svg>
                                </span>
                            {/if}
                            {#if c.health && c.health !== "none"}
                                <div class="flex items-center p-1.5 bg-slate-100 dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 text-slate-500" title={`Health: ${c.health}`}>
                                    <!-- Heartbeat/Medicine icon -->
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 {healthColor(c.health).replace('bg-', 'text-')} {c.health === 'starting' ? 'animate-pulse' : ''}" viewBox="0 0 20 20" fill="currentColor">
                                        <path fill-rule="evenodd" d="M3.172 5.172a4 4 0 015.656 0L10 6.343l1.172-1.171a4 4 0 115.656 5.656L10 17.657l-6.828-6.829a4 4 0 010-5.656z" clip-rule="evenodd" />
                                    </svg>
                                </div>
                            {/if}
                            <span class="p-1.5 rounded-lg {stateColor(c.state)}" title={`Status: ${c.state}`}>
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                                    {#if c.state.toLowerCase() === 'running'}
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd" />
                                    {:else}
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v6a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd" />
                                    {/if}
                                </svg>
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
                            <div class="flex items-center gap-1.5 px-2 py-0.5 bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-400 rounded-md" title={`Policy: ${getPolicy(c.labels)}`}>
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                                    <path d="M11 3a1 1 0 10-2 0v1a1 1 0 102 0V3zM15.657 5.757a1 1 0 00-1.414-1.414l-.707.707a1 1 0 001.414 1.414l.707-.707zM18 10a1 1 0 01-1 1h-1a1 1 0 110-2h1a1 1 0 011 1zM5.05 6.464A1 1 0 106.464 5.05l-.707-.707a1 1 0 00-1.414 1.414l.707.707zM5 10a1 1 0 01-1 1H3a1 1 0 110-2h1a1 1 0 011 1zM8 16v-1h4v1a2 2 0 11-4 0zM12 14H8a2 2 0 002 2 2 2 0 002-2z" />
                                </svg>
                                <span class="text-[9px] font-black uppercase tracking-tighter">{getPolicy(c.labels)}</span>
                            </div>
                        {:else}
                            <div class="flex items-center gap-1.5 px-2 py-0.5 bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400 rounded-md" title="Manual Control Only">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M10 2a4 4 0 00-4 4v1H5a1 1 0 00-.994.89l-1 9A1 1 0 004 18h12a1 1 0 00.994-1.11l-1-9A1 1 0 0015 7h-1V6a4 4 0 00-4-4zm2 5V6a2 2 0 10-4 0v1h4zm-6 3a1 1 0 112 0 1 1 0 01-2 0zm7-1a1 1 0 100 2 1 1 0 000-2z" clip-rule="evenodd" />
                                </svg>
                                <span class="text-[9px] font-black uppercase tracking-tighter">Manual</span>
                            </div>
                        {/if}
                    </div>

                    {#if intelNeedsAttention(c)}
                        <div class="rounded-lg border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-900/10 px-2.5 py-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-rose-700 dark:text-rose-300">AI Automation Needs Metadata</p>
                            <p class="mt-1 text-[10px] text-rose-700/90 dark:text-rose-200/90">{intelPrimaryIssue(c) || "Open Manage > Intelligence and configure repository/changelog overrides."}</p>
                        </div>
                    {/if}
                    {#if hasAIBlocked(c)}
                        <div class="rounded-lg border border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-900/10 px-2.5 py-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-amber-700 dark:text-amber-300">AI Blocked Last Upgrade</p>
                            <p class="mt-1 text-[10px] text-amber-800/90 dark:text-amber-200/90">{lookupAIBlocked(c)?.reason || "Risk threshold exceeded."}</p>
                            <button
                                onclick={() => onNavigate("container-detail", { id: c.id, tab: "lifecycle" })}
                                class="mt-2 text-[10px] font-black uppercase tracking-widest text-amber-700 dark:text-amber-300 hover:underline"
                            >
                                Review Lifecycle History
                            </button>
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
                            onclick={() => handleTriggerScan(c.id)}
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
