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
    let logPane = $state<HTMLElement | null>(null);
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let activeRequestController: AbortController | null = null;
    let latestRequestId = 0;
    let requestKey = $state("");

    function findContainer(): ContainerSummary | undefined {
        return containers.find((entry) => entry.id === id);
    }

    function containerName(): string {
        const match = findContainer();
        return match?.names?.[0]?.replace(/^\//, "") || (id.length > 12 ? id.slice(0, 12) : id);
    }

    function containerState(): string {
        return findContainer()?.state || "unknown";
    }

    function containerHealth(): string {
        return findContainer()?.health || "unknown";
    }

    function containerImage(): string {
        return findContainer()?.image || "Unknown image";
    }

    function stateTone(state: string): string {
        switch (state.toLowerCase()) {
            case "running":
                return "bg-emerald-500/15 text-emerald-300 border-emerald-500/30";
            case "paused":
                return "bg-amber-500/15 text-amber-300 border-amber-500/30";
            case "exited":
                return "bg-rose-500/15 text-rose-300 border-rose-500/30";
            default:
                return "bg-slate-500/15 text-slate-300 border-slate-500/30";
        }
    }

    function healthTone(health: string): string {
        switch (health.toLowerCase()) {
            case "healthy":
                return "bg-emerald-400";
            case "unhealthy":
                return "bg-rose-400";
            case "starting":
                return "bg-amber-400";
            default:
                return "bg-slate-500";
        }
    }

    function isNearBottom(): boolean {
        if (!logPane) return true;
        const distance = logPane.scrollHeight - logPane.scrollTop - logPane.clientHeight;
        return distance < 80;
    }

    function scrollToBottom() {
        setTimeout(() => {
            if (logPane) {
                logPane.scrollTop = logPane.scrollHeight;
            }
        }, 0);
    }

    async function loadLogs(forceScroll = false) {
        const stickToBottom = forceScroll || (live && isNearBottom());
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
            if (stickToBottom) {
                scrollToBottom();
            }
        } catch (err) {
            if (controller.signal.aborted || requestId !== latestRequestId) return;
            error = err instanceof Error ? err.message : "Failed to load container logs";
        } finally {
            if (activeRequestController === controller) {
                activeRequestController = null;
            }
            if (requestId === latestRequestId) {
                loading = false;
            }
        }
    }

    function stopPolling() {
        if (pollTimer) {
            clearInterval(pollTimer);
            pollTimer = null;
        }
    }

    function startPolling() {
        stopPolling();
        if (!live) return;
        pollTimer = setInterval(() => {
            loadLogs(false);
        }, pollIntervalMs);
    }

    function pauseLiveUpdates() {
        live = false;
        stopPolling();
    }

    function resumeLiveUpdates() {
        live = true;
        loadLogs(true);
        startPolling();
    }

    function refreshLogs() {
        loadLogs(true);
    }

    function navigateBackToFleet() {
        onNavigate(backRoute, backParams ?? {});
    }

    $effect(() => {
        const nextKey = `${id}|${tail}|${timestamps}`;
        if (requestKey === nextKey) return;
        requestKey = nextKey;
        logs = null;
        void loadLogs(true);
    });

    $effect(() => {
        if (live) {
            startPolling();
        } else {
            stopPolling();
        }
        return () => stopPolling();
    });

    onDestroy(() => {
        stopPolling();
        activeRequestController?.abort();
    });
</script>

<div class="space-y-6 min-h-[calc(100vh-10rem)]">
    <section class="rounded-[2rem] border border-slate-200 dark:border-slate-800 bg-[radial-gradient(circle_at_top_left,_rgba(45,212,191,0.12),_transparent_32%),linear-gradient(135deg,_rgba(15,23,42,0.98),_rgba(2,6,23,0.94))] text-white shadow-2xl overflow-hidden">
        <div class="sticky top-0 z-10 border-b border-white/10 bg-slate-950/85 backdrop-blur-xl">
            <div class="px-5 py-4 md:px-8 md:py-6 flex flex-col gap-4">
                <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div class="space-y-3 min-w-0">
                        <button
                            onclick={navigateBackToFleet}
                            class="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/5 px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-200 hover:bg-white/10"
                        >
                            <span aria-hidden="true">←</span>
                            Fleet
                        </button>
                        <div>
                            <p class="text-[11px] font-black uppercase tracking-[0.28em] text-cyan-200/80">Container Logs</p>
                            <h1 class="mt-2 text-2xl md:text-4xl font-black tracking-tight break-words">{containerName()}</h1>
                            <p class="mt-2 text-sm text-slate-300 break-all">{containerImage()}</p>
                        </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-2 lg:justify-end">
                        <span class={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] ${stateTone(containerState())}`}>
                            <span>{containerState()}</span>
                        </span>
                        <span class="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-200">
                            <span class={`h-2.5 w-2.5 rounded-full ${healthTone(containerHealth())}`}></span>
                            <span>{containerHealth()}</span>
                        </span>
                        <span class={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] ${live ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-200' : 'border-amber-400/30 bg-amber-400/10 text-amber-200'}`}>
                            <span class={`h-2 w-2 rounded-full ${live ? 'bg-emerald-300 animate-pulse' : 'bg-amber-300'}`}></span>
                            <span>{live ? 'Live' : 'Paused'}</span>
                        </span>
                    </div>
                </div>

                <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
                    <div class="flex flex-wrap items-center gap-2">
                        {#if live}
                            <button
                                onclick={pauseLiveUpdates}
                                class="rounded-xl border border-amber-400/30 bg-amber-400/10 px-4 py-2 text-[10px] font-black uppercase tracking-[0.22em] text-amber-100 hover:bg-amber-400/20"
                            >
                                Pause
                            </button>
                        {:else}
                            <button
                                onclick={resumeLiveUpdates}
                                class="rounded-xl border border-emerald-400/30 bg-emerald-400/10 px-4 py-2 text-[10px] font-black uppercase tracking-[0.22em] text-emerald-100 hover:bg-emerald-400/20"
                            >
                                Resume
                            </button>
                        {/if}
                        <button
                            onclick={refreshLogs}
                            class="rounded-xl border border-white/15 bg-white/5 px-4 py-2 text-[10px] font-black uppercase tracking-[0.22em] text-slate-100 hover:bg-white/10"
                        >
                            Refresh
                        </button>
                    </div>
                    <div class="flex flex-wrap items-center gap-3 text-[11px] text-slate-300">
                        <label class="flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-3 py-2">
                            <span class="font-black uppercase tracking-[0.18em] text-[10px] text-slate-400">Tail</span>
                            <select bind:value={tail} class="bg-transparent text-slate-100 outline-none">
                                {#each tailOptions as option}
                                    <option value={option}>{option} lines</option>
                                {/each}
                            </select>
                        </label>
                        <label class="flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-3 py-2">
                            <input bind:checked={timestamps} type="checkbox" class="h-4 w-4 rounded border-white/20 bg-slate-950 text-cyan-400 focus:ring-cyan-400" />
                            <span class="font-black uppercase tracking-[0.18em] text-[10px] text-slate-300">Timestamps</span>
                        </label>
                        {#if logs}
                            <span class="font-mono text-[10px] text-slate-400">Captured {new Date(logs.capturedAt * 1000).toLocaleTimeString()}</span>
                        {/if}
                    </div>
                </div>
            </div>
        </div>

        <div class="px-4 pb-4 md:px-6 md:pb-6">
            {#if error}
                <div class="mx-1 mt-4 rounded-2xl border border-rose-500/25 bg-rose-500/10 px-4 py-3 text-sm text-rose-100">
                    {error}
                </div>
            {/if}

            <div bind:this={logPane} class="mt-4 min-h-[60vh] max-h-[72vh] overflow-auto rounded-[1.5rem] border border-white/10 bg-[#0a0f1a] px-4 py-4 font-mono text-[11px] leading-relaxed text-slate-200 whitespace-pre-wrap shadow-inner shadow-black/30">
                {#if loading && !logs}
                    <div class="flex min-h-[55vh] items-center justify-center text-slate-500">Loading logs...</div>
                {:else if logs && logs.combined}
                    {logs.combined}
                {:else if logs}
                    No logs available.
                {:else}
                    Waiting for logs.
                {/if}
            </div>

            <div class="mt-3 flex flex-wrap items-center justify-between gap-3 px-1 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400">
                <span>{logs?.lineCount || 0} lines</span>
                {#if logs?.truncated}
                    <span class="text-amber-300">Showing latest {logs.tail} lines</span>
                {/if}
            </div>
        </div>
    </section>
</div>
