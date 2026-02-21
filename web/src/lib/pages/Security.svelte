<script lang="ts">
    import { onMount } from "svelte";
    import type { MalwareScanDetail, ScanJobStatus, ScanStartResponse, ScanSummary, TrivyScanDetails } from "../api-types";
    import MalwareScanPanel from "../components/MalwareScanPanel.svelte";
    import TrivyFindingsPanel from "../components/TrivyFindingsPanel.svelte";

    let { params } = $props<{
        params?: { target: string };
    }>();

    // Component State
    let summary = $state<ScanSummary | null>(null);
    let activeJob = $state<ScanJobStatus | null>(null);
    let scanError = $state("");
    let target = $state("nginx:latest");
    let malwareTarget = $state("/var/lib/docker");
    let pollTimer: number | null = null;
    let cancellingJob = $state(false);
    let vulnerabilityDetails = $state<TrivyScanDetails | null>(null);
    let loadingVulnerabilityDetails = $state(false);
    let malwareDetails = $state<MalwareScanDetail[]>([]);
    let loadingMalwareDetails = $state(false);

    $effect(() => {
        if (params?.target) {
            target = params.target;
        }
    });

    const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";
    const isTerminalJobStatus = (status: string) => status === "completed" || status === "failed" || status === "cancelled";

    async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
        const response = await fetch(url, init);
        if (!response.ok) throw new Error(`${url} failed (${response.status})`);
        return (await response.json()) as T;
    }

    async function loadData() {
        try {
            const [vuln, mal] = await Promise.all([
                fetch("/api/scans/summary").then(r => r.ok ? r.json() : null),
                fetch("/api/scans/malware/summary").then(r => r.ok ? r.json() : [])
            ]);
            summary = vuln;
            void mal;
            await Promise.all([loadVulnerabilityDetails(target), loadMalwareDetails()]);
            if (vulnerabilityDetails?.summary) {
                summary = vulnerabilityDetails.summary;
            }
        } catch (e) {
            console.error("Failed to load security data", e);
        }
    }

    async function loadVulnerabilityDetails(scanTarget: string) {
        const normalized = scanTarget.trim();
        if (!normalized) {
            vulnerabilityDetails = null;
            return;
        }
        loadingVulnerabilityDetails = true;
        try {
            const res = await fetch(`/api/scans/details?target=${encodeURIComponent(normalized)}`);
            if (!res.ok) {
                vulnerabilityDetails = null;
                return;
            }
            vulnerabilityDetails = await res.json();
        } catch (e) {
            vulnerabilityDetails = null;
        } finally {
            loadingVulnerabilityDetails = false;
        }
    }

    async function loadMalwareDetails() {
        const normalized = malwareTarget.trim();
        if (!normalized) {
            malwareDetails = [];
            return;
        }
        loadingMalwareDetails = true;
        try {
            const res = await fetch(`/api/scans/malware/details?target=${encodeURIComponent(normalized)}&limit=25`);
            if (!res.ok) {
                malwareDetails = [];
                return;
            }
            const rows = await res.json();
            malwareDetails = Array.isArray(rows) ? rows : [];
        } catch (e) {
            malwareDetails = [];
        } finally {
            loadingMalwareDetails = false;
        }
    }

    async function pollJob(jobId: string) {
        if (pollTimer) window.clearInterval(pollTimer);
        pollTimer = window.setInterval(async () => {
            try {
                const job = await fetchJSON<ScanJobStatus>(`/api/scans/jobs/${jobId}`);
                activeJob = job;
                if (isTerminalJobStatus(job.status)) {
                    if (pollTimer) window.clearInterval(pollTimer);
                    pollTimer = null;
                    await loadData();
                }
            } catch (e) {
                scanError = "Failed to poll scan status";
            }
        }, 2000);
    }

    async function startScan() {
        scanError = "";
        try {
            const response = await fetchJSON<ScanStartResponse>("/api/scans/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ target })
            });
            activeJob = { jobId: response.jobId, target, status: response.status, source: "trivy", progress: 0, startedAt: Math.floor(Date.now() / 1000) };
            await pollJob(response.jobId);
        } catch (e) {
            scanError = "Vulnerability scan failed to start";
        }
    }

    async function startMalwareScan() {
        scanError = "";
        try {
            const response = await fetchJSON<ScanStartResponse>("/api/scans/malware/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ target: malwareTarget })
            });
            activeJob = { jobId: response.jobId, target: malwareTarget, status: response.status, source: "clamav", progress: 0, startedAt: Math.floor(Date.now() / 1000) };
            await pollJob(response.jobId);
        } catch (e) {
            scanError = "Malware scan failed to start";
        }
    }

    async function stopActiveJob() {
        if (!activeJob || activeJob.status !== "running" || cancellingJob) return;
        cancellingJob = true;
        scanError = "";
        try {
            const cancelled = await fetchJSON<ScanJobStatus>(`/api/scans/jobs/${activeJob.jobId}/cancel`, {
                method: "POST"
            });
            activeJob = cancelled;
            if (pollTimer) {
                window.clearInterval(pollTimer);
                pollTimer = null;
            }
            await loadData();
        } catch (e) {
            scanError = e instanceof Error ? e.message : "Failed to stop scan job";
        } finally {
            cancellingJob = false;
        }
    }

    onMount(() => {
        loadData();
        return () => {
            if (pollTimer) window.clearInterval(pollTimer);
        };
    });
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Security Suite</h2>
    </div>

    {#if scanError}
        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
            {scanError}
        </div>
    {/if}

    {#if activeJob}
        <div class="p-4 bg-brand-50 border border-brand-100 dark:bg-brand-900/10 dark:border-brand-900/30 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-4">
                {#if activeJob.status === "running"}
                    <div class="w-8 h-8 rounded-full border-2 border-brand-600 border-t-transparent animate-spin"></div>
                {:else}
                    <div class="w-8 h-8 rounded-full border-2 border-slate-300 dark:border-slate-600"></div>
                {/if}
                <div>
                    <div class="text-sm font-bold text-brand-900 dark:text-brand-300">
                        {#if activeJob.status === "running"}
                            Scanning {activeJob.target}...
                        {:else if activeJob.status === "cancelled"}
                            Scan cancelled for {activeJob.target}
                        {:else}
                            Scan {activeJob.status} for {activeJob.target}
                        {/if}
                    </div>
                    <div class="text-xs text-brand-600 uppercase font-black tracking-widest">{activeJob.source} {activeJob.status}</div>
                </div>
            </div>
            <div class="flex items-center gap-3">
                {#if activeJob.status === "running"}
                    <button
                        onclick={stopActiveJob}
                        disabled={cancellingJob}
                        class="px-3 py-1.5 rounded-lg border border-rose-200 text-rose-700 bg-white hover:bg-rose-50 disabled:opacity-50 text-xs font-black uppercase tracking-widest transition-colors"
                    >
                        {cancellingJob ? "Stopping..." : "Stop"}
                    </button>
                {/if}
                <span class="text-xs font-mono text-slate-400">JOB: {activeJob.jobId}</span>
            </div>
        </div>
    {/if}

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <!-- Vulnerability Scanner -->
        <section class="space-y-4">
            <div class="flex items-center gap-3 mb-2">
                <div class="p-2 bg-amber-100 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400 rounded-lg">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                </div>
                <h3 class="text-lg font-bold text-slate-900 dark:text-white">Image Vulnerabilities (Trivy)</h3>
            </div>

            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
                <div class="flex gap-2">
                    <input 
                        bind:value={target} 
                        placeholder="image:tag" 
                        class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500 outline-none transition-all"
                    />
                    <button 
                        onclick={startScan}
                        disabled={activeJob?.status === "running"}
                        class="px-6 py-2 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold text-sm transition-all shadow-lg shadow-brand-500/20"
                    >
                        Scan
                    </button>
                </div>

                {#if summary}
                    <div class="space-y-4">
                        <div class="flex items-center justify-between">
                            <span class="text-sm font-medium text-slate-500 italic truncate max-w-[250px]">{summary.target}</span>
                            <span class="px-2 py-1 bg-slate-100 dark:bg-slate-700 rounded text-[10px] font-bold uppercase">{riskBand(summary.riskScore)} Risk</span>
                        </div>
                        <div class="pt-4 border-t border-slate-50 dark:border-slate-700 flex justify-between items-center text-[10px] text-slate-400 font-bold uppercase tracking-widest">
                            <span>Last Scan: {new Date(summary.scannedAt * 1000).toLocaleString()}</span>
                            <span>Score: {summary.riskScore}/100</span>
                        </div>
                    </div>
                {:else}
                    <div class="py-8 text-center text-slate-400 italic text-sm">No vulnerability data for this target</div>
                {/if}

                <div class="pt-2 border-t border-slate-100 dark:border-slate-700">
                    <TrivyFindingsPanel details={vulnerabilityDetails} loading={loadingVulnerabilityDetails} />
                </div>
            </div>
        </section>

        <!-- Malware Scanner -->
        <section class="space-y-4">
            <div class="flex items-center gap-3 mb-2">
                <div class="p-2 bg-rose-100 dark:bg-rose-900/20 text-rose-600 dark:text-rose-400 rounded-lg">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                </div>
                <h3 class="text-lg font-bold text-slate-900 dark:text-white">Malware Shield (ClamAV)</h3>
            </div>

            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
                <div class="flex gap-2">
                    <input 
                        bind:value={malwareTarget} 
                        placeholder="/path/to/scan" 
                        class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500 outline-none transition-all"
                    />
                    <button 
                        onclick={startMalwareScan}
                        disabled={activeJob?.status === "running"}
                        class="px-6 py-2 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold text-sm transition-all shadow-lg shadow-brand-500/20"
                    >
                        Scan
                    </button>
                </div>

                <div class="space-y-3">
                    <MalwareScanPanel details={malwareDetails} loading={loadingMalwareDetails} />
                </div>
            </div>
        </section>
    </div>
</div>
