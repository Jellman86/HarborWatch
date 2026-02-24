<script lang="ts">
    import { onMount } from "svelte";
    import type { ImageSummary } from "../api-types";
    import PaginationBar from "../components/PaginationBar.svelte";
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

    interface SchedulerLogRow {
        id?: number;
        timestamp?: number;
        source?: string;
        message?: string;
    }

    function cleanupEventRank(message: string): number {
        const text = message.toLowerCase();
        if (text.includes("manually triggering task: docker_system_prune")) return 10;
        if (text.includes("starting docker system prune task")) return 20;
        if (/pruned .*images/i.test(message)) return 30;
        if (/pruned .*containers/i.test(message)) return 40;
        if (text.includes("manual task completed: docker_system_prune")) return 90;
        if (text.includes("manual task docker_system_prune failed")) return 90;
        return 50;
    }

    function consumeSchedulerLogs(rows: SchedulerLogRow[]): { sawOutcome: boolean; sawTerminal: boolean } {
        let sawOutcome = false;
        let sawTerminal = false;
        const ordered = [...rows].sort((a, b) => {
            const tsA = Number(a?.timestamp || 0);
            const tsB = Number(b?.timestamp || 0);
            if (tsA !== tsB) return tsA - tsB;
            const idA = Number(a?.id || 0);
            const idB = Number(b?.id || 0);
            if (idA !== idB) return idA - idB;
            return cleanupEventRank(String(a?.message || "")) - cleanupEventRank(String(b?.message || ""));
        });
        for (const entry of ordered) {
            const message = String(entry?.message || "").trim();
            const source = String(entry?.source || "").toLowerCase();
            if (!message || !source.includes("scheduler")) continue;
            const tracked = /prune|docker_system_prune|space reclaimed/i.test(message);
            if (!tracked) continue;
            const id = Number(entry?.id || 0);
            const ts = Number(entry?.timestamp || Math.floor(Date.now() / 1000));
            const key = id > 0 ? `id:${id}` : `${ts}:${message}`;
            if (cleanupSeen.has(key)) continue;
            cleanupSeen.add(key);
            const level =
                /manual task completed/i.test(message) ? "success"
                : /error|failed/i.test(message) ? "error"
                : "info";
            appendCleanupLog(level, message, ts);
            if (/Pruned .*images|Pruned .*containers|image prune failed|container prune failed/i.test(message)) {
                sawOutcome = true;
            }
            if (/Manual task completed: docker_system_prune|Manual task docker_system_prune failed/i.test(message)) {
                sawTerminal = true;
            }
        }
        return { sawOutcome, sawTerminal };
    }

    async function pollCleanupFeedback(startedAt: number) {
        let sawOutcome = false;
        for (let i = 0; i < 15; i++) {
            await new Promise((resolve) => setTimeout(resolve, 2500));
            try {
                const res = await fetch(`/api/system/logs?limit=250&source=scheduler&since=${startedAt}`);
                if (!res.ok) continue;
                const logs = await res.json();
                const result = consumeSchedulerLogs(Array.isArray(logs) ? logs : []);
                if (result.sawOutcome) sawOutcome = true;
                if (result.sawTerminal) {
                    appendCleanupLog("success", "Cleanup workflow completed.");
                    return;
                }
            } catch {
                // Keep polling to avoid transient log endpoint errors.
            }
        }
        if (sawOutcome) {
            appendCleanupLog("info", "Prune actions finished; waiting for final scheduler status log.");
            return;
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
    
    let searchQuery = $state("");
    let sortBy = $state<"repo" | "size" | "security">("repo");
    let pageIndex = $state(0);
    const pageSize = 24;

    const filteredRows = $derived(
        safeRows.filter(row => {
            if (!searchQuery.trim()) return true;
            const q = searchQuery.toLowerCase();
            return (
                imageRepo(row).toLowerCase().includes(q) ||
                imageQualifier(row).toLowerCase().includes(q) ||
                formatId(row.id).toLowerCase().includes(q)
            );
        }).sort((a, b) => {
            if (sortBy === "size") return b.size - a.size;
            if (sortBy === "security") {
                const score = (r: ImageIntelligenceRow) => (r.vulnerabilityCritical || 0) * 10 + (r.vulnerabilityHigh || 0) * 5 + (r.vulnerabilityTotal || 0);
                return score(b) - score(a);
            }
            return imageRepo(a).localeCompare(imageRepo(b));
        })
    );

    const totalPages = $derived(Math.max(1, Math.ceil(filteredRows.length / pageSize)));
    const pagedRows = $derived(filteredRows.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));
    const pageStart = $derived(filteredRows.length === 0 ? 0 : (pageIndex * pageSize) + 1);
    const pageEnd = $derived(Math.min((pageIndex + 1) * pageSize, filteredRows.length));

    const stats = $derived({
        total: safeRows.length,
        totalSize: safeRows.reduce((acc, row) => acc + row.size, 0),
        inUse: safeRows.filter(row => !row.pruneCandidate).length,
        vulnerable: safeRows.filter(row => (row.vulnerabilityTotal || 0) > 0 || row.malwareInfected).length
    });

    const imageRepo = (row: ImageIntelligenceRow) => parseImageRef(row.repoTags?.[0] || row.primaryRef || "").repository;
    const imageQualifier = (row: ImageIntelligenceRow) => parseImageRef(row.repoTags?.[0] || row.primaryRef || "").qualifier || ":latest";
    const digestOnlyCount = $derived(safeRows.filter((row) => isDigestOnlyArtifact(row)).length);

    function isDigestOnlyArtifact(img: ImageIntelligenceRow): boolean {
        const ref = String(img.repoTags?.[0] || img.primaryRef || "").trim().toLowerCase();
        return ref.startsWith("sha256:");
    }

    function securitySummary(img: ImageIntelligenceRow): string {
        if (img.malwareInfected) return `Malware ${img.malwareThreatCount || 0}`;
        if ((img.vulnerabilityCritical || 0) > 0) return `Critical CVE ${img.vulnerabilityCritical}`;
        if ((img.vulnerabilityHigh || 0) > 0) return `High CVE ${img.vulnerabilityHigh}`;
        if ((img.vulnerabilityTotal || 0) > 0) return `CVE ${img.vulnerabilityTotal}`;
        if ((img.securityScannedAt || 0) > 0) return "No findings";
        return "Not scanned";
    }

    function lifecycleSummary(img: ImageIntelligenceRow): string {
        if (isDigestOnlyArtifact(img)) {
            return img.pruneCandidate
                ? "Digest artifact (may be retained by Docker references)"
                : "Digest artifact in use";
        }
        if (img.pruneCandidate) return "Prune next run";
        if (img.outdated) return "Outdated";
        return "In use";
    }

    function formatWhen(ts?: number): string {
        const n = Number(ts || 0);
        if (!Number.isFinite(n) || n <= 0) return "Unknown";
        return new Date(n * 1000).toLocaleString();
    }

    function imageReferenceCount(img: ImageIntelligenceRow): number {
        const refs = new Set<string>();
        for (const tag of img.repoTags || []) {
            const value = String(tag || "").trim();
            if (value && value !== "<none>:<none>") refs.add(value);
        }
        const primary = String(img.primaryRef || "").trim();
        if (primary && primary !== "<none>:<none>") refs.add(primary);
        return refs.size;
    }

    $effect(() => {
        searchQuery;
        sortBy;
        safeRows.length;
        pageIndex = 0;
    });

    $effect(() => {
        if (pageIndex > totalPages - 1) {
            pageIndex = Math.max(0, totalPages - 1);
        }
    });
</script>

<div class="space-y-6">
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-6 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Image Repository</h2>
            <p class="text-xs text-slate-500 font-medium">Local artifact storage and versioning history.</p>
        </div>
        
        <div class="flex flex-wrap items-center gap-3">
            <div class="flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-2.5 py-1.5 min-w-[200px]">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <input
                    bind:value={searchQuery}
                    placeholder="Search images..."
                    class="bg-transparent text-[11px] text-slate-700 dark:text-slate-200 outline-none w-full"
                />
            </div>

            <select 
                bind:value={sortBy}
                class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-1.5 text-[10px] font-black uppercase tracking-widest outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-600 dark:text-slate-300"
            >
                <option value="repo">Sort: Repository</option>
                <option value="size">Sort: Size</option>
                <option value="security">Sort: Security Risk</option>
            </select>

            <div class="flex items-center gap-2">
                <button 
                    onclick={() => loadImages()}
                    class="p-2 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 text-slate-400 hover:text-brand-600 transition-colors flex items-center justify-center shadow-sm"
                    title="Refresh Registry"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                </button>
                <button 
                    onclick={pruneImages}
                    disabled={pruning}
                    class="px-4 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-[10px] font-black uppercase tracking-widest transition-all shadow-lg shadow-rose-500/20 disabled:opacity-50 flex items-center justify-center gap-2"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                    {pruning ? 'Pruning...' : 'Cleanup'}
                </button>
            </div>
        </div>
    </div>

    <!-- Quick Stats Bar -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 opacity-0 animate-reveal" style="animation-delay: 0.05s">
        <div class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-4 rounded-2xl shadow-sm text-left">
            <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Total Images</span>
            <p class="text-xl font-black text-slate-900 dark:text-white mt-1">{stats.total}</p>
        </div>
        <div class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-4 rounded-2xl shadow-sm text-left">
            <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Storage Footprint</span>
            <p class="text-xl font-black text-slate-900 dark:text-white mt-1">{formatSize(stats.totalSize)}</p>
        </div>
        <div class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-4 rounded-2xl shadow-sm text-left">
            <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Active Artifacts</span>
            <p class="text-xl font-black text-emerald-600 mt-1">{stats.inUse}</p>
        </div>
        <div class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-4 rounded-2xl shadow-sm text-left">
            <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Security Risks</span>
            <p class="text-xl font-black {stats.vulnerable > 0 ? 'text-rose-600' : 'text-slate-400'} mt-1">{stats.vulnerable}</p>
        </div>
    </div>

    {#if cleanupLogVisible}
        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/70 p-4 space-y-3 shadow-xl">
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
            <div class="w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest">Syncing with Registry...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-12 text-center animate-reveal">
            <div class="w-16 h-16 bg-rose-100 dark:bg-rose-900/30 rounded-full flex items-center justify-center mx-auto text-rose-600 dark:text-rose-400 mb-4">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
            </div>
            <p class="text-rose-600 font-bold uppercase tracking-widest text-xs">{error}</p>
            <button onclick={() => loadImages()} class="mt-4 px-4 py-2 bg-rose-100 dark:bg-rose-900/30 text-rose-700 dark:text-rose-300 rounded-xl text-[10px] font-black uppercase tracking-widest hover:bg-rose-200 transition-colors">Retry</button>
        </div>
    {:else if filteredRows.length === 0}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-16 text-center animate-reveal">
            <p class="text-slate-400 italic font-medium">No images matched your filter criteria.</p>
            <button onclick={() => searchQuery = ""} class="mt-4 text-brand-600 text-[10px] font-black uppercase tracking-widest hover:underline">Clear Search</button>
        </div>
    {:else}
        <PaginationBar
            summaryText={`Showing ${pageStart}-${pageEnd} of ${filteredRows.length}`}
            pageText={`Page ${pageIndex + 1}/${totalPages}`}
            canPrev={pageIndex > 0}
            canNext={pageIndex < totalPages - 1}
            onPrev={() => pageIndex = Math.max(0, pageIndex - 1)}
            onNext={() => pageIndex = Math.min(totalPages - 1, pageIndex + 1)}
        />
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-5 opacity-0 animate-reveal stagger-1">
            {#each pagedRows as img, i}
                <article
                    class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-xl hover:border-brand-500/50 transition-all flex flex-col group overflow-hidden opacity-0 animate-reveal"
                    style="animation-delay: {0.1 + (i * 0.03)}s"
                >
                    <div class="p-5 flex-1 flex flex-col gap-4">
                        <div class="flex items-start justify-between gap-3 text-left">
                            <div class="min-w-0 flex-1">
                                <h3 class="text-sm font-black text-slate-900 dark:text-white truncate group-hover:text-brand-600 transition-colors" title={imageRepo(img)}>
                                    {imageRepo(img)}
                                </h3>
                                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mt-0.5">
                                    {imageQualifier(img)}
                                </p>
                            </div>
                            <div class="flex-shrink-0 text-right">
                                <p class="text-[10px] font-black text-slate-900 dark:text-white">{formatSize(img.size)}</p>
                                <p class="text-[9px] font-mono text-slate-400 mt-0.5 uppercase">{formatId(img.id)}</p>
                            </div>
                        </div>

                        <div class="space-y-2 text-left">
                            <div class="flex flex-col gap-1.5">
                                <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Security Posture</span>
                                <div class="flex flex-wrap gap-1.5">
                                    {#if img.malwareInfected}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 border border-rose-200 dark:border-rose-800/50">Malware {img.malwareThreatCount || 0}</span>
                                    {:else if (img.vulnerabilityCritical || 0) > 0}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 border border-rose-200 dark:border-rose-800/50">Critical {img.vulnerabilityCritical}</span>
                                    {:else if (img.vulnerabilityHigh || 0) > 0}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300 border border-orange-200 dark:border-orange-800/50">High Risk {img.vulnerabilityHigh}</span>
                                    {:else if (img.vulnerabilityTotal || 0) > 0}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300 border border-amber-200 dark:border-amber-800/50">CVE {img.vulnerabilityTotal}</span>
                                    {:else if (img.securityScannedAt || 0) > 0}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/50">Healthy</span>
                                    {:else}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400">Unscanned</span>
                                    {/if}
                                </div>
                            </div>

                            <div class="flex flex-col gap-1.5">
                                <span class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Artifact Lifecycle</span>
                                <div class="flex flex-wrap gap-1.5">
                                    {#if isDigestOnlyArtifact(img)}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-slate-100 dark:bg-slate-900/40 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700/50">Digest Only</span>
                                    {/if}
                                    {#if img.outdated}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300 border border-amber-200 dark:border-amber-800/50">Outdated</span>
                                    {/if}
                                    {#if img.pruneCandidate}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-rose-50 dark:bg-rose-900/10 text-rose-600 dark:text-rose-400 border border-rose-100 dark:border-rose-900/30">Prune Candidate</span>
                                    {:else}
                                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase bg-brand-50 dark:bg-brand-900/20 text-brand-700 dark:text-brand-300 border border-brand-100 dark:border-brand-900/30">In Use</span>
                                    {/if}
                                </div>
                            </div>

                            <div class="rounded-xl border border-slate-100 dark:border-slate-700/60 bg-slate-50 dark:bg-slate-900/30 p-3 space-y-2">
                                <p class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Artifact Detail</p>
                                <div class="grid grid-cols-2 gap-2 text-[10px]">
                                    <div>
                                        <p class="text-slate-400 uppercase tracking-widest text-[8px] font-black">References</p>
                                        <p class="text-slate-700 dark:text-slate-200 font-bold">{imageReferenceCount(img)}</p>
                                    </div>
                                    <div>
                                        <p class="text-slate-400 uppercase tracking-widest text-[8px] font-black">Usage</p>
                                        <p class="text-slate-700 dark:text-slate-200 font-bold">{img.inUse ? "Active" : "Unused"}</p>
                                    </div>
                                    <div class="col-span-2">
                                        <p class="text-slate-400 uppercase tracking-widest text-[8px] font-black">Last Security Scan</p>
                                        <p class="text-slate-700 dark:text-slate-200 font-bold">{formatWhen(img.securityScannedAt)}</p>
                                    </div>
                                    <div class="col-span-2">
                                        <p class="text-slate-400 uppercase tracking-widest text-[8px] font-black">Primary Reference</p>
                                        <p class="text-slate-700 dark:text-slate-200 font-mono break-all">{img.primaryRef || img.repoTags?.[0] || img.id}</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="px-5 py-3 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700/50 flex justify-between items-center">
                        <span class="text-[9px] font-bold text-slate-500 uppercase tracking-widest">{securitySummary(img)}</span>
                        <span class="text-[9px] font-bold text-slate-400 uppercase">{lifecycleSummary(img)}</span>
                    </div>
                </article>
            {/each}
        </div>
    {/if}
</div>
