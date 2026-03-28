<script lang="ts">
    import { onDestroy } from "svelte";
    import type { ContainerLogs, ContainerSummary } from "../api-types";

    let { id, containers, onNavigate, backRoute = "containers", backParams = null } = $props<{
        id: string;
        containers: ContainerSummary[];
        onNavigate: (route: string, params?: any) => void;
        backRoute?: string;
        backParams?: Record<string, unknown> | null;
    }>();

    const tailOptions = [100, 200, 500, 1000, 2000];
    const pollIntervalMs = 3000;

    let logs = $state<ContainerLogs | null>(null);
    let loading = $state(true);
    let error = $state("");
    let live = $state(true);
    let timestamps = $state(false);
    let tail = $state(200);
    let filterText = $state("");
    let following = $state(true); // auto-scroll to bottom
    let logPane = $state<HTMLElement | null>(null);
    let filterInput = $state<HTMLInputElement | null>(null);
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let activeRequestController: AbortController | null = null;
    let latestRequestId = 0;
    let requestKey = $state("");
    let actionBusy = $state<string | null>(null); // "start" | "stop" | "restart"

    // ── Container helpers ──────────────────────────────────────────────────────
    function findContainer(): ContainerSummary | undefined {
        return containers.find((entry) => entry.id === id);
    }

    const containerName = $derived(
        findContainer()?.names?.[0]?.replace(/^\//, "") || (id.length > 12 ? id.slice(0, 12) : id)
    );
    const containerState = $derived(findContainer()?.state || "unknown");
    const containerHealth = $derived(findContainer()?.health || "none");
    const containerImage = $derived(findContainer()?.image || "");

    function stateBadgeClass(state: string): string {
        switch (state.toLowerCase()) {
            case "running": return "border-emerald-500/40 bg-emerald-500/15 text-emerald-300";
            case "paused":  return "border-amber-500/40 bg-amber-500/15 text-amber-300";
            case "exited":  return "border-rose-500/40 bg-rose-500/15 text-rose-300";
            default:        return "border-slate-600/40 bg-slate-500/15 text-slate-400";
        }
    }

    function healthDot(health: string): string {
        switch (health.toLowerCase()) {
            case "healthy":   return "bg-emerald-400";
            case "unhealthy": return "bg-rose-400";
            case "starting":  return "bg-amber-400 animate-pulse";
            default:          return "bg-slate-600";
        }
    }

    // ── Log parsing ────────────────────────────────────────────────────────────
    const ANSI_RE = /\x1B\[[0-9;]*[mGKHFJPXST]/g;

    function stripAnsi(s: string): string {
        return s.replace(ANSI_RE, "");
    }

    function lineColor(line: string): string {
        const u = line.toUpperCase();
        if (/\b(ERROR|FATAL|CRITICAL|CRIT|PANIC|EXCEPTION)\b/.test(u)) return "text-rose-400";
        if (/\b(WARN|WARNING)\b/.test(u))   return "text-amber-400";
        if (/\b(DEBUG|TRACE|VERBOSE)\b/.test(u)) return "text-slate-500";
        if (/\b(SUCCESS|DONE|READY|STARTED|LISTENING)\b/.test(u)) return "text-emerald-400";
        return "text-slate-300";
    }

    const allLines = $derived<string[]>(
        logs?.combined
            ? logs.combined.split("\n").map(stripAnsi).filter((l) => l.trim() !== "")
            : []
    );

    const filteredLines = $derived<string[]>(
        filterText.trim()
            ? allLines.filter((l) => l.toLowerCase().includes(filterText.toLowerCase()))
            : allLines
    );

    const matchCount = $derived(filterText.trim() ? filteredLines.length : null);

    // ── Scroll behaviour ───────────────────────────────────────────────────────
    function isNearBottom(): boolean {
        if (!logPane) return true;
        return logPane.scrollHeight - logPane.scrollTop - logPane.clientHeight < 100;
    }

    function scrollToBottom() {
        setTimeout(() => { if (logPane) logPane.scrollTop = logPane.scrollHeight; }, 0);
    }

    function onScroll() {
        if (!logPane) return;
        following = isNearBottom();
    }

    // When filtered lines change and we're following, scroll to bottom
    $effect(() => {
        filteredLines; // track
        if (following) scrollToBottom();
    });

    // ── Data loading ───────────────────────────────────────────────────────────
    async function loadLogs(forceScroll = false) {
        const stickToBottom = forceScroll || following;
        const requestId = ++latestRequestId;
        activeRequestController?.abort();
        const controller = new AbortController();
        activeRequestController = controller;
        loading = logs === null;
        error = "";
        try {
            const params = new URLSearchParams({ tail: String(tail) });
            if (timestamps) params.set("timestamps", "1");
            const res = await fetch(`/api/docker/${id}/logs?${params.toString()}`, { signal: controller.signal });
            if (!res.ok) {
                const data = await res.json().catch(() => ({}));
                throw new Error(data.error || data.message || `Request failed (${res.status})`);
            }
            const nextLogs = (await res.json()) as ContainerLogs;
            if (controller.signal.aborted || requestId !== latestRequestId) return;
            logs = nextLogs;
            if (stickToBottom) { following = true; scrollToBottom(); }
        } catch (err) {
            if (controller.signal.aborted || requestId !== latestRequestId) return;
            error = err instanceof Error ? err.message : "Failed to load container logs";
        } finally {
            if (activeRequestController === controller) activeRequestController = null;
            if (requestId === latestRequestId) loading = false;
        }
    }

    function stopPolling() {
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
    }

    function startPolling() {
        stopPolling();
        if (!live) return;
        pollTimer = setInterval(() => loadLogs(false), pollIntervalMs);
    }

    function pauseLive() { live = false; stopPolling(); }

    function resumeLive() { live = true; following = true; loadLogs(true); startPolling(); }

    function jumpToBottom() { following = true; scrollToBottom(); }

    // ── Container actions ──────────────────────────────────────────────────────
    async function containerAction(action: "start" | "stop" | "restart") {
        if (actionBusy) return;
        actionBusy = action;
        try {
            await fetch(`/api/docker/${id}/${action}`, { method: "POST" });
            setTimeout(() => loadLogs(false), 800);
        } catch (_) {
            // silent — container list will reflect state on next poll
        } finally {
            actionBusy = null;
        }
    }

    // ── Keyboard shortcut ──────────────────────────────────────────────────────
    function onKeydown(e: KeyboardEvent) {
        if ((e.ctrlKey || e.metaKey) && e.key === "f") {
            e.preventDefault();
            filterInput?.focus();
        }
        if (e.key === "Escape" && filterText) {
            filterText = "";
        }
    }

    // ── Effects ────────────────────────────────────────────────────────────────
    $effect(() => {
        const nextKey = `${id}|${tail}|${timestamps}`;
        if (requestKey === nextKey) return;
        requestKey = nextKey;
        logs = null;
        void loadLogs(true);
    });

    $effect(() => {
        if (live) { startPolling(); } else { stopPolling(); }
        return () => stopPolling();
    });

    onDestroy(() => {
        stopPolling();
        activeRequestController?.abort();
    });
</script>

<svelte:window onkeydown={onKeydown} />

<!--
    Negative margins bleed to the content-shell edges so the viewer
    can use the full available height without double-padding.
-->
<div class="log-shell flex flex-col bg-slate-950 text-slate-100 -mx-4 -my-4 md:-mx-8 md:-my-8 overflow-hidden">

    <!-- ── Top bar ─────────────────────────────────────────────────────────── -->
    <header class="flex-none flex flex-col gap-0 border-b border-white/10 bg-slate-950/95 backdrop-blur-xl">

        <!-- Row 1: nav + identity + badges + actions -->
        <div class="flex items-center gap-3 px-4 py-3 md:px-6 min-w-0 flex-wrap">
            <button
                onclick={() => onNavigate(backRoute, backParams ?? {})}
                class="flex-none inline-flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-300 hover:bg-white/10 transition-colors"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M9.707 16.707a1 1 0 01-1.414 0l-6-6a1 1 0 010-1.414l6-6a1 1 0 011.414 1.414L5.414 9H17a1 1 0 110 2H5.414l4.293 4.293a1 1 0 010 1.414z" clip-rule="evenodd"/>
                </svg>
                Back
            </button>

            <!-- Container name + image -->
            <div class="flex-1 min-w-0">
                <div class="flex items-baseline gap-2 flex-wrap">
                    <span class="text-[10px] font-black uppercase tracking-widest text-cyan-400/70">Logs</span>
                    <h1 class="text-sm font-black tracking-tight text-white truncate">{containerName}</h1>
                    {#if containerImage}
                        <span class="text-[10px] text-slate-500 truncate hidden sm:block">{containerImage}</span>
                    {/if}
                </div>
            </div>

            <!-- State / health badges -->
            <div class="flex items-center gap-2 flex-wrap">
                <span class={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[10px] font-black uppercase tracking-widest ${stateBadgeClass(containerState)}`}>
                    {containerState}
                </span>
                {#if containerHealth !== "none" && containerHealth !== "unknown"}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/5 px-2.5 py-1 text-[10px] font-black uppercase tracking-widest text-slate-300">
                        <span class={`h-1.5 w-1.5 rounded-full ${healthDot(containerHealth)}`}></span>
                        {containerHealth}
                    </span>
                {/if}
                <!-- Live indicator -->
                <span class={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[10px] font-black uppercase tracking-widest transition-colors ${live ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300' : 'border-amber-500/30 bg-amber-500/10 text-amber-300'}`}>
                    <span class={`h-1.5 w-1.5 rounded-full ${live ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'}`}></span>
                    {live ? "Live" : "Paused"}
                </span>
            </div>

            <!-- Container quick actions -->
            <div class="flex items-center gap-1.5 flex-none">
                {#if containerState === "running"}
                    <button
                        onclick={() => containerAction("restart")}
                        disabled={!!actionBusy}
                        title="Restart"
                        class="rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-300 hover:bg-white/10 disabled:opacity-40 transition-colors"
                    >
                        {actionBusy === "restart" ? "…" : "Restart"}
                    </button>
                    <button
                        onclick={() => containerAction("stop")}
                        disabled={!!actionBusy}
                        title="Stop"
                        class="rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-rose-300 hover:bg-rose-500/20 disabled:opacity-40 transition-colors"
                    >
                        {actionBusy === "stop" ? "…" : "Stop"}
                    </button>
                {:else if containerState === "exited" || containerState === "created"}
                    <button
                        onclick={() => containerAction("start")}
                        disabled={!!actionBusy}
                        title="Start"
                        class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-emerald-300 hover:bg-emerald-500/20 disabled:opacity-40 transition-colors"
                    >
                        {actionBusy === "start" ? "…" : "Start"}
                    </button>
                {/if}
            </div>
        </div>

        <!-- Row 2: toolbar -->
        <div class="flex flex-wrap items-center gap-2 border-t border-white/5 px-4 py-2 md:px-6">
            <!-- Search / filter -->
            <div class="relative flex-1 min-w-[160px] max-w-xs">
                <svg xmlns="http://www.w3.org/2000/svg" class="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-500 pointer-events-none" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"/>
                </svg>
                <input
                    bind:this={filterInput}
                    bind:value={filterText}
                    type="text"
                    placeholder="Filter logs… (Ctrl+F)"
                    class="w-full rounded-lg border border-white/10 bg-white/5 pl-8 pr-3 py-1.5 text-[11px] text-slate-200 placeholder-slate-600 outline-none focus:border-cyan-500/50 focus:bg-white/8 transition-colors font-mono"
                />
                {#if filterText}
                    <button
                        onclick={() => filterText = ""}
                        class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300"
                        aria-label="Clear filter"
                    >×</button>
                {/if}
            </div>

            <div class="flex items-center gap-2 flex-wrap">
                <!-- Tail selector -->
                <label class="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5">
                    <span class="text-[10px] font-black uppercase tracking-widest text-slate-500">Tail</span>
                    <select bind:value={tail} class="bg-transparent text-[11px] text-slate-300 outline-none">
                        {#each tailOptions as option}
                            <option value={option}>{option}</option>
                        {/each}
                    </select>
                </label>

                <!-- Timestamps toggle -->
                <label class="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5 cursor-pointer">
                    <input bind:checked={timestamps} type="checkbox" class="h-3 w-3 rounded border-white/20 bg-slate-900 accent-cyan-400" />
                    <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">Timestamps</span>
                </label>

                <!-- Pause / Resume -->
                {#if live}
                    <button
                        onclick={pauseLive}
                        class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-amber-300 hover:bg-amber-500/20 transition-colors"
                    >Pause</button>
                {:else}
                    <button
                        onclick={resumeLive}
                        class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-emerald-300 hover:bg-emerald-500/20 transition-colors"
                    >Resume</button>
                {/if}

                <!-- Refresh -->
                <button
                    onclick={() => loadLogs(true)}
                    class="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-300 hover:bg-white/10 transition-colors"
                >Refresh</button>
            </div>
        </div>
    </header>

    <!-- ── Log pane ────────────────────────────────────────────────────────── -->
    <div
        bind:this={logPane}
        onscroll={onScroll}
        class="flex-1 overflow-y-auto overflow-x-auto font-mono text-[11px] leading-relaxed"
        style="background: #080d14;"
    >
        {#if loading && !logs}
            <div class="flex h-full min-h-[200px] items-center justify-center text-slate-600 text-xs tracking-widest uppercase font-black">
                Loading logs…
            </div>
        {:else if error}
            <div class="m-4 rounded-xl border border-rose-500/25 bg-rose-500/8 px-4 py-3 text-sm text-rose-300">
                {error}
            </div>
        {:else if filteredLines.length === 0}
            <div class="flex h-full min-h-[200px] items-center justify-center text-slate-600 text-xs tracking-widest uppercase font-black">
                {filterText ? "No lines match filter" : "No logs available"}
            </div>
        {:else}
            <div class="py-2">
                {#each filteredLines as line, i}
                    <div class={`flex gap-3 px-4 py-[1px] hover:bg-white/[0.03] ${lineColor(line)}`}>
                        <span class="flex-none w-10 text-right text-slate-700 select-none tabular-nums">{i + 1}</span>
                        <span class="flex-1 whitespace-pre-wrap break-all">{line}</span>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- ── Footer status bar ───────────────────────────────────────────────── -->
    <footer class="flex-none flex items-center justify-between gap-4 border-t border-white/8 bg-slate-950/95 px-4 py-2 md:px-6 text-[10px] font-black uppercase tracking-widest text-slate-600">
        <div class="flex items-center gap-4">
            <span>{allLines.length} lines</span>
            {#if matchCount !== null}
                <span class="text-cyan-500">{matchCount} match{matchCount !== 1 ? 'es' : ''}</span>
            {/if}
            {#if logs?.truncated}
                <span class="text-amber-500">Showing latest {logs.tail}</span>
            {/if}
            {#if logs}
                <span>Captured {new Date(logs.capturedAt * 1000).toLocaleTimeString()}</span>
            {/if}
        </div>
        <div class="flex items-center gap-3">
            {#if !following}
                <button
                    onclick={jumpToBottom}
                    class="flex items-center gap-1.5 rounded-full border border-cyan-500/30 bg-cyan-500/10 px-3 py-1 text-cyan-400 hover:bg-cyan-500/20 transition-colors normal-case tracking-normal font-bold text-[10px]"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"/>
                    </svg>
                    Jump to bottom
                </button>
            {:else}
                <span class="text-emerald-600 normal-case tracking-normal">Following</span>
            {/if}
        </div>
    </footer>
</div>

<style>
    .log-shell {
        /* Fill the full viewport height minus the sidebar-adjusted top offset */
        height: calc(100dvh - 9.5rem); /* mobile: header ~4.5rem + py-4 padding */
    }

    @media (min-width: 768px) {
        .log-shell {
            height: calc(100dvh - 4rem); /* desktop: py-8 top + bottom padding */
        }
    }
</style>
