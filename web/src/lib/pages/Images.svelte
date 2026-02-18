<script lang="ts">
    import { onMount } from "svelte";
    import type { ImageSummary } from "../api-types";
    import { toasts } from "../stores/ToastStore";
    import { parseImageRef } from "../utils/image-ref";

    let { images = $bindable([]) } = $props<{ images: ImageSummary[] }>();
    let loading = $state(false);
    let error = $state("");
    let pruning = $state(false);
    let cleanupLogVisible = $state(false);
    let cleanupLog = $state<Array<{ ts: number; level: "info" | "success" | "error"; message: string }>>([]);
    let cleanupSeen = $state(new Set<string>());

    interface ImageIntelligenceRow extends ImageSummary {
        primaryRef?: string;
        inUse?: boolean;
        outdated?: boolean;
        pruneCandidate?: boolean;
        vulnerabilityTotal?: number;
        vulnerabilityCritical?: number;
        vulnerabilityHigh?: number;
        malwareInfected?: boolean;
        malwareThreatCount?: number;
        securityScannedAt?: number;
    }

    let imageRows = $state<ImageIntelligenceRow[]>([]);

    function appendCleanupLog(level: "info" | "success" | "error", message: string, ts = Math.floor(Date.now() / 1000)) {
        cleanupLog = [...cleanupLog, { ts, level, message }];
    }

    function consumeSchedulerLogs(rows: Array<{ timestamp: number; source: string; message: string }>): boolean {
        let sawCompletion = false;
        const ordered = [...rows].sort((a, b) => a.timestamp - b.timestamp);
        for (const entry of ordered) {
            const message = String(entry?.message || "").trim();
            const source = String(entry?.source || "").toLowerCase();
            if (!message || !source.includes("scheduler")) continue;
            const tracked = /prune|docker_system_prune|space reclaimed/i.test(message);
            if (!tracked) continue;
            const key = `${entry.timestamp}:${message}`;
            if (cleanupSeen.has(key)) continue;
            cleanupSeen.add(key);
            const level = /error|failed/i.test(message) ? "error" : "info";
            appendCleanupLog(level, message, entry.timestamp || Math.floor(Date.now() / 1000));
            if (/Pruned .*images|Pruned .*containers|image prune failed|container prune failed/i.test(message)) {
                sawCompletion = true;
            }
        }
        return sawCompletion;
    }

    async function pollCleanupFeedback(startedAt: number) {
        for (let i = 0; i < 15; i++) {
            await new Promise((resolve) => setTimeout(resolve, 2500));
            try {
                const res = await fetch(`/api/system/logs?limit=250&source=scheduler&since=${startedAt}`);
                if (!res.ok) continue;
                const logs = await res.json();
                if (consumeSchedulerLogs(Array.isArray(logs) ? logs : [])) {
                    appendCleanupLog("success", "Cleanup workflow completed.");
                    return;
                }
            } catch {
                // Keep polling to avoid transient log endpoint errors.
            }
        }
        appendCleanupLog("info", "Cleanup still running in background. Refresh logs shortly.");
    }

    async function loadImages(opts?: { silent?: boolean }) {
        const silent = !!opts?.silent;
        if (!silent) {
            loading = true;
            error = "";
        }
        try {
            const res = await fetch("/api/docker/images/intelligence");
            if (res.ok) {
                const data = await res.json();
                const rows = Array.isArray(data) ? data : [];
                imageRows = rows;
                images = rows;
            } else {
                if (!silent) {
                    error = "Failed to load images";
                }
            }
        } catch (e) {
            if (!silent) {
                error = "Could not connect to backend";
            }
        } finally {
            if (!silent) {
                loading = false;
            }
        }
    }

    async function pruneImages() {
        if (!confirm("Are you sure you want to trigger a system-wide image prune? This will remove all unused images.")) return;
        const startedAt = Math.floor(Date.now() / 1000);
        cleanupLogVisible = true;
        cleanupLog = [];
        cleanupSeen = new Set<string>();
        appendCleanupLog("info", "Submitting cleanup request...");

        pruning = true;
        try {
            const res = await fetch("/api/docker/prune", { method: "POST" });
            if (res.ok) {
                appendCleanupLog("success", "Cleanup task triggered (docker_system_prune).");
                await pollCleanupFeedback(startedAt);
                await loadImages();
                toasts.success("Repository cleanup triggered.");
            } else {
                const body = await res.json().catch(() => ({}));
                const msg = body?.message || `Cleanup request failed (${res.status})`;
                appendCleanupLog("error", msg);
                toasts.error(msg);
            }
        } catch (e) {
            appendCleanupLog("error", "Cleanup request failed due to connection error.");
            toasts.error("Cleanup request failed.");
        } finally {
            pruning = false;
        }
    }

    onMount(() => {
        if (images.length > 0) {
            imageRows = images as ImageIntelligenceRow[];
            void loadImages({ silent: true });
            return;
        }
        void loadImages();
    });

    const formatSize = (bytes: number) => {
        const mb = bytes / (1024 * 1024);
        return mb > 1024 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(1)} MB`;
    };

    const formatId = (id: string) => id.replace('sha256:', '').slice(0, 12);
    const safeRows = $derived((imageRows && imageRows.length > 0) ? imageRows : (images as ImageIntelligenceRow[] || []));
    const imageRepo = (row: ImageIntelligenceRow) => parseImageRef(row.repoTags?.[0] || row.primaryRef || "").repository;
    const imageQualifier = (row: ImageIntelligenceRow) => parseImageRef(row.repoTags?.[0] || row.primaryRef || "").qualifier || ":latest";

    function securitySummary(img: ImageIntelligenceRow): string {
        if (img.malwareInfected) return `Malware ${img.malwareThreatCount || 0}`;
        if ((img.vulnerabilityCritical || 0) > 0) return `Critical CVE ${img.vulnerabilityCritical}`;
        if ((img.vulnerabilityHigh || 0) > 0) return `High CVE ${img.vulnerabilityHigh}`;
        if ((img.vulnerabilityTotal || 0) > 0) return `CVE ${img.vulnerabilityTotal}`;
        if ((img.securityScannedAt || 0) > 0) return "No findings";
        return "Not scanned";
    }

    function lifecycleSummary(img: ImageIntelligenceRow): string {
        if (img.pruneCandidate) return "Prune next run";
        if (img.outdated) return "Outdated";
        return "In use";
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Image Repository</h2>
            <p class="text-xs text-slate-500 font-medium">Local artifact storage and versioning history.</p>
        </div>
        <div class="flex items-center gap-3">
            <button 
                onclick={loadImages}
                class="p-2 text-slate-400 hover:text-brand-600 transition-colors"
                title="Refresh Registry"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
            </button>
            <button 
                onclick={pruneImages}
                disabled={pruning}
                class="px-4 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-[10px] font-black uppercase tracking-widest transition-all shadow-lg shadow-rose-500/20 disabled:opacity-50 flex items-center gap-2"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                {pruning ? 'Pruning...' : 'Cleanup Repository'}
            </button>
        </div>
    </div>

    {#if cleanupLogVisible}
        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/70 p-4 space-y-3">
            <div class="flex items-center justify-between gap-3">
                <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Cleanup Log</p>
                <button
                    onclick={() => cleanupLogVisible = false}
                    class="px-2 py-1 rounded-lg border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-900/40"
                >Hide</button>
            </div>
            <div class="max-h-[160px] overflow-y-auto rounded-xl bg-slate-50 dark:bg-slate-900/30 border border-slate-200 dark:border-slate-700 p-3 space-y-2">
                {#if cleanupLog.length === 0}
                    <p class="text-[11px] text-slate-500 italic">No cleanup events yet.</p>
                {:else}
                    {#each cleanupLog as entry}
                        <div class="text-[11px] flex items-start gap-2">
                            <span class="font-mono text-slate-400 min-w-[72px]">{new Date(entry.ts * 1000).toLocaleTimeString()}</span>
                            <span class={entry.level === "error" ? "text-rose-600 dark:text-rose-300" : entry.level === "success" ? "text-emerald-600 dark:text-emerald-300" : "text-slate-600 dark:text-slate-300"}>{entry.message}</span>
                        </div>
                    {/each}
                {/if}
            </div>
        </div>
    {/if}

    {#if loading}
        <div class="flex flex-col items-center justify-center py-20 gap-4 text-slate-400">
            <div class="w-8 h-8 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest">Scanning Registry...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-12 text-center animate-reveal">
            <p class="text-rose-600 font-bold uppercase tracking-widest text-xs">{error}</p>
        </div>
    {:else if safeRows.length === 0}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-16 text-center animate-reveal">
            <p class="text-slate-400 italic font-medium">No images found in local repository.</p>
        </div>
    {:else}
        <div class="md:hidden space-y-3 opacity-0 animate-reveal stagger-1">
            {#each safeRows as img, i}
                <article
                    class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl p-4 space-y-2 opacity-0 animate-reveal"
                    style="animation-delay: {0.08 + (i * 0.02)}s"
                >
                    <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                            <p class="text-xs font-bold text-slate-900 dark:text-white truncate">{imageRepo(img)}</p>
                            <p class="text-[10px] font-black text-brand-600 uppercase tracking-widest">{imageQualifier(img)}</p>
                        </div>
                        <p class="text-[10px] font-mono text-slate-500">{formatSize(img.size)}</p>
                    </div>
                    <div class="flex flex-wrap gap-1.5">
                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-slate-100 text-slate-700 dark:bg-slate-900/40 dark:text-slate-200">{securitySummary(img)}</span>
                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-slate-100 text-slate-700 dark:bg-slate-900/40 dark:text-slate-200">{lifecycleSummary(img)}</span>
                    </div>
                    <p class="text-[10px] text-slate-400 font-mono">{formatId(img.id)}</p>
                </article>
            {/each}
        </div>

        <div class="hidden md:block bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden opacity-0 animate-reveal stagger-1">
            <table class="w-full text-left border-collapse">
                <thead>
                    <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest border-b border-slate-100 dark:border-slate-700">
                        <th class="px-8 py-4">Artifact ID</th>
                        <th class="px-8 py-4">Repository / Tag</th>
                        <th class="px-8 py-4">Security</th>
                        <th class="px-8 py-4">Lifecycle</th>
                        <th class="px-8 py-4 text-right">Storage Size</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                    {#each safeRows as img, i}
                        <tr 
                            class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors opacity-0 animate-reveal"
                            style="animation-delay: {0.1 + (i * 0.03)}s"
                        >
                            <td class="px-8 py-4 font-mono text-[10px] text-slate-400">{formatId(img.id)}</td>
                            <td class="px-8 py-4">
                                <div class="flex flex-col">
                                    <span class="font-bold text-slate-900 dark:text-white truncate max-w-[300px]">{imageRepo(img)}</span>
                                    <span class="text-[10px] font-black text-brand-600 uppercase tracking-tighter">{imageQualifier(img)}</span>
                                </div>
                            </td>
                            <td class="px-8 py-4">
                                <div class="flex flex-wrap gap-1.5">
                                    {#if img.malwareInfected}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300">Malware {img.malwareThreatCount || 0}</span>
                                    {/if}
                                    {#if (img.vulnerabilityCritical || 0) > 0}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300">Critical CVE {img.vulnerabilityCritical}</span>
                                    {:else if (img.vulnerabilityHigh || 0) > 0}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300">High CVE {img.vulnerabilityHigh}</span>
                                    {:else if (img.vulnerabilityTotal || 0) > 0}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">CVE {img.vulnerabilityTotal}</span>
                                    {:else if (img.securityScannedAt || 0) > 0}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">No findings</span>
                                    {:else}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300">Not scanned</span>
                                    {/if}
                                </div>
                            </td>
                            <td class="px-8 py-4">
                                <div class="flex flex-wrap gap-1.5">
                                    {#if img.outdated}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">Outdated</span>
                                    {/if}
                                    {#if img.pruneCandidate}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300">Prune Next Run</span>
                                    {:else}
                                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">In Use</span>
                                    {/if}
                                </div>
                            </td>
                            <td class="px-8 py-4 font-mono text-[10px] text-slate-500 text-right">{formatSize(img.size)}</td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>
