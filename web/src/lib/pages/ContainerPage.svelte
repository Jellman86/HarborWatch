<script lang="ts">
    import { onMount } from "svelte";
    import type { ContainerDetail, MalwareScanDetail, TrivyScanDetails } from "../api-types";
    import MetricChart from "../components/MetricChart.svelte";
    import DiskUsagePanel from "../components/DiskUsagePanel.svelte";
    import MalwareScanPanel from "../components/MalwareScanPanel.svelte";
    import TrivyFindingsPanel from "../components/TrivyFindingsPanel.svelte";
    import { toasts } from "../stores/ToastStore";

    let { id, onNavigate } = $props<{
        id: string;
        onNavigate: (route: string, params?: any) => void;
    }>();

    let detail = $state<ContainerDetail | null>(null);
    let configYaml = $state("");
    let aiAudit = $state("");
    let loading = $state(true);
    let auditing = $state(false);
    let savingRules = $state(false);
    let runningTrivyScan = $state(false);
    let runningMalwareScan = $state(false);
    let activeTab = $state("insights");
    let error = $state("");
    let scanMessage = $state("");
    let lifecycleMessage = $state("");
    let vulnerabilityDetails = $state<TrivyScanDetails | null>(null);
    let loadingVulnerabilityDetails = $state(false);
    let malwareDetails = $state<MalwareScanDetail[]>([]);
    let loadingMalwareDetails = $state(false);

    async function loadDetail(silent = false) {
        if (!silent) {
            loading = true;
            error = "";
        }
        try {
            const res = await fetch(`/api/docker/${id}`);
            if (res.ok) {
                const data = await res.json();
                // Defensive defaulting: ensure arrays exist before assignment
                data.malwareSummary = data.malwareSummary || [];
                data.actionHistory = data.actionHistory || [];
                data.recentMetrics = data.recentMetrics || [];
                if (data.rules) {
                    if (typeof data.rules.inheritAutomation !== "boolean") data.rules.inheritAutomation = true;
                    if (typeof data.rules.upgradesAutomation !== "boolean") data.rules.upgradesAutomation = true;
                    if (typeof data.rules.maintenanceAutomation !== "boolean") data.rules.maintenanceAutomation = true;
                    if (typeof data.rules.securityAutomation !== "boolean") data.rules.securityAutomation = true;
                }
                detail = data;
                await Promise.all([
                    loadVulnerabilityDetails(data?.summary?.image || ""),
                    loadMalwareDetailsForContainer()
                ]);
            } else {
                const body = await res.json().catch(() => ({}));
                if (!silent) {
                    error = body?.message || `Container lookup failed (${res.status})`;
                }
                vulnerabilityDetails = null;
                malwareDetails = [];
            }
        } catch (e) {
            if (!silent) {
                error = "Failed to load container data";
            }
            vulnerabilityDetails = null;
            malwareDetails = [];
        } finally {
            if (!silent) {
                loading = false;
            }
        }
    }

    async function loadVulnerabilityDetails(target: string) {
        const normalized = target?.trim();
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

    async function loadMalwareDetailsForContainer() {
        loadingMalwareDetails = true;
        try {
            const prefix = `container:${id}`;
            const res = await fetch(`/api/scans/malware/details?prefix=${encodeURIComponent(prefix)}&limit=40`);
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

    async function saveRules() {
        if (!detail?.rules) return;
        lifecycleMessage = "";
        savingRules = true;
        try {
            const res = await fetch(`/api/docker/${id}/rules`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(detail.rules)
            });
            if (res.ok) {
                await loadDetail(true);
                lifecycleMessage = detail?.rules?.validateUrl
                    ? `Validation URL in use: ${detail.rules.validateUrl}`
                    : "Lifecycle policy updated.";
                toasts.success("Lifecycle policy updated.");
            } else {
                const body = await res.json().catch(() => ({}));
                toasts.error(body?.message || `Failed to save lifecycle policy (${res.status})`);
            }
        } catch (e) {
            toasts.error("Failed to save lifecycle policy.");
        } finally {
            savingRules = false;
        }
    }

    async function runAudit() {
        auditing = true;
        aiAudit = "";
        try {
            const res = await fetch(`/api/ai/audit-compose/${id}`);
            if (res.ok) {
                const data = await res.json();
                configYaml = data.config;
                aiAudit = data.analysis;
            }
        } catch (e) {
            console.error("Audit failed", e);
        } finally {
            auditing = false;
        }
    }

    async function triggerTrivyScan() {
        if (!detail || runningTrivyScan) return;
        runningTrivyScan = true;
        scanMessage = "Starting Trivy scan...";
        try {
            const res = await fetch("/api/scans/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ target: detail.summary.image })
            });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `scan failed (${res.status})`);
            }
            const payload = await res.json();
            const jobId = payload?.jobId;
            if (!jobId) {
                throw new Error("scan job id was not returned");
            }
            toasts.info("Trivy scan started.");
            scanMessage = "Trivy scan is running...";
            const done = await waitForScanJob(jobId);
            if (done === "completed") {
                toasts.success("Trivy scan completed.");
                scanMessage = "Trivy scan completed.";
                await loadDetail(true);
                return;
            }
            if (done === "failed") {
                toasts.error("Trivy scan failed.");
                scanMessage = "Trivy scan failed.";
                return;
            }
            toasts.warning("Trivy scan is still running in the background.");
            scanMessage = "Trivy scan still running.";
        } catch (e) {
            scanMessage = "Failed to start Trivy scan.";
            toasts.error(e instanceof Error ? e.message : "Failed to start Trivy scan.");
        } finally {
            runningTrivyScan = false;
        }
    }

    async function waitForScanJob(jobId: string): Promise<"completed" | "failed" | "running"> {
        const maxPolls = 40;
        for (let i = 0; i < maxPolls; i++) {
            await new Promise((resolve) => setTimeout(resolve, 1500));
            const res = await fetch(`/api/scans/jobs/${jobId}`);
            if (!res.ok) continue;
            const job = await res.json();
            const status = String(job?.status || "");
            if (status === "completed") return "completed";
            if (status === "failed") return "failed";
        }
        return "running";
    }

    async function triggerContainerMalwareScan() {
        if (runningMalwareScan) return;
        runningMalwareScan = true;
        scanMessage = "Queueing ClamAV container scan...";
        const previousTop = detail?.malwareSummary?.[0]?.scannedAt || 0;
        try {
            const res = await fetch(`/api/scans/malware/container/${id}`, { method: "POST" });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `scan failed (${res.status})`);
            }
            toasts.success("ClamAV scans queued for container rootfs and mounts.");
            scanMessage = "ClamAV scan queued. Waiting for results...";
            await refreshMalwareHistory(previousTop);
        } catch (e) {
            scanMessage = "Failed to queue ClamAV scan.";
            toasts.error(e instanceof Error ? e.message : "Failed to queue ClamAV scan.");
        } finally {
            runningMalwareScan = false;
        }
    }

    async function refreshMalwareHistory(previousTopScannedAt: number) {
        for (let i = 0; i < 20; i++) {
            await new Promise((resolve) => setTimeout(resolve, 3000));
            await loadDetail(true);
            const top = detail?.malwareSummary?.[0]?.scannedAt || 0;
            if (top > previousTopScannedAt) {
                scanMessage = "ClamAV results updated.";
                toasts.success("ClamAV scan finished and results were added.");
                return;
            }
        }
        scanMessage = "ClamAV scan is still running in background.";
        toasts.info("ClamAV scan is still running. Results will appear when complete.");
    }

    function openManualUpdate() {
        if (!detail) return;
        onNavigate("updates", {
            containerId: detail.summary.id,
            targetImage: detail.summary.image,
            validateUrl: detail.rules?.validateUrl || ""
        });
    }

    onMount(() => {
        loadDetail();
    });

    const stateColor = (state: string) => {
        switch(state?.toLowerCase()) {
            case 'running': return 'bg-emerald-500';
            case 'exited': return 'bg-rose-500';
            default: return 'bg-slate-500';
        }
    };
</script>

<div class="space-y-8">
    <!-- Breadcrumbs/Back -->
    <button 
        onclick={() => onNavigate('containers')}
        class="flex items-center gap-2 text-slate-500 hover:text-brand-600 transition-colors group"
    >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 transition-transform group-hover:-translate-x-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
        </svg>
        <span class="text-xs font-black uppercase tracking-widest">Back to Fleet</span>
    </button>

    {#if loading}
        <div class="flex flex-col items-center justify-center py-20 gap-4">
            <div class="w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest text-slate-400">Syncing with Docker...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 p-8 rounded-2xl text-center">
            <p class="text-rose-600 dark:text-rose-400 font-bold">{error}</p>
        </div>
    {:else if detail}
        <!-- Header -->
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-6 bg-white dark:bg-slate-800 p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl shadow-slate-200/50 dark:shadow-none">
            <div class="flex items-center gap-6">
                <div class="relative">
                    <div class="w-20 h-20 rounded-2xl bg-brand-600 flex items-center justify-center text-white shadow-lg shadow-brand-500/20">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                        </svg>
                    </div>
                    <div class="absolute -bottom-1 -right-1 w-6 h-6 rounded-full border-4 border-white dark:border-slate-800 {stateColor(detail.summary.state)}"></div>
                </div>
                <div>
                    <div class="flex items-center gap-3">
                        <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tight">
                            {detail.summary.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                        </h2>
                        {#if detail.summary.updateAvailable}
                            <span class="px-2 py-1 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded text-[10px] font-black uppercase animate-pulse">
                                Update Available
                            </span>
                        {/if}
                    </div>
                    <p class="text-slate-500 font-mono text-sm mt-1">{detail.summary.image}</p>
                </div>
            </div>

            <div class="flex gap-2">
                <button class="px-6 py-3 bg-slate-900 dark:bg-white text-white dark:text-slate-900 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg hover:scale-105 transition-all">
                    Restart
                </button>
                <button class="px-6 py-3 bg-rose-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-rose-500/20 hover:scale-105 transition-all">
                    Stop
                </button>
            </div>
        </div>

        <!-- Navigation Tabs -->
        <div class="flex gap-1 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl w-fit border border-slate-200 dark:border-slate-800">
            {#each ['insights', 'security', 'lifecycle', 'configuration'] as tab}
                <button 
                    onclick={() => activeTab = tab}
                    class="px-6 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeTab === tab ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                >
                    {tab}
                </button>
            {/each}
        </div>

        <!-- Tab Content -->
        <div class="min-h-[400px]">
            {#if activeTab === 'insights'}
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                        <MetricChart metrics={detail.recentMetrics || []} title="CPU Utilization" type="cpu" />
                    </div>
                    <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                        <MetricChart metrics={detail.recentMetrics || []} title="Memory footprint" type="memory" />
                    </div>
                    <div class="lg:col-span-2 bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                        <h3 class="text-sm font-black uppercase tracking-tight text-slate-400 mb-4">Container disk usage</h3>
                        <DiskUsagePanel diskUsage={detail.diskUsage} />
                    </div>
                </div>
            {:else if activeTab === 'security'}
                <div class="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
                        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 p-6 rounded-3xl">
                            <span class="text-[10px] font-black uppercase text-rose-600 dark:text-rose-400">Critical Risks</span>
                            <div class="text-4xl font-black text-rose-700 dark:text-rose-300 mt-2">{vulnerabilityDetails?.summary.critical ?? detail.vulnerabilitySummary?.critical ?? 0}</div>
                        </div>
                        <div class="bg-orange-50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/30 p-6 rounded-3xl">
                            <span class="text-[10px] font-black uppercase text-orange-600 dark:text-orange-400">High Risks</span>
                            <div class="text-4xl font-black text-orange-700 dark:text-orange-300 mt-2">{vulnerabilityDetails?.summary.high ?? detail.vulnerabilitySummary?.high ?? 0}</div>
                        </div>
                        <div class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-6 rounded-3xl flex flex-col justify-center items-center gap-3">
                            <button
                                onclick={triggerTrivyScan}
                                disabled={runningTrivyScan}
                                class="w-full py-3 bg-brand-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20 hover:bg-brand-700 transition-all disabled:opacity-50"
                            >
                                {runningTrivyScan ? 'Scanning...' : 'Run Trivy Scan'}
                            </button>
                            <button
                                onclick={triggerContainerMalwareScan}
                                disabled={runningMalwareScan}
                                class="w-full py-3 bg-emerald-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-emerald-500/20 hover:bg-emerald-700 transition-all disabled:opacity-50"
                            >
                                {runningMalwareScan ? 'Queueing...' : 'Scan with ClamAV'}
                            </button>
                            {#if scanMessage}
                                <p class="text-[10px] text-center text-slate-500">{scanMessage}</p>
                            {/if}
                        </div>
                    </div>

                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
                        <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm p-6 h-full">
                            <h3 class="text-sm font-black uppercase tracking-tight text-slate-400 mb-4">Vulnerability details</h3>
                            <TrivyFindingsPanel details={vulnerabilityDetails} loading={loadingVulnerabilityDetails} />
                        </div>

                        <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden h-full">
                            <div class="p-6 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center">
                                <h3 class="text-sm font-black uppercase tracking-tight text-slate-400">Malware scan details</h3>
                            </div>
                            <div class="p-6">
                                <MalwareScanPanel details={malwareDetails} loading={loadingMalwareDetails} />
                            </div>
                        </div>
                    </div>
                </div>
            {:else if activeTab === 'lifecycle'}
                <div class="grid grid-cols-1 md:grid-cols-2 gap-8 items-start animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="bg-white dark:bg-slate-800 p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-8">
                        <div>
                            <h3 class="text-lg font-black text-slate-900 dark:text-white uppercase tracking-tight">Asset Lifecycle Policy</h3>
                            <p class="text-xs text-slate-500 mt-1">Define how HarborWatch manages updates for this specific container.</p>
                        </div>
                        
                        {#if detail.rules}
                            <div class="space-y-6">
                                <div class="space-y-2">
                                    <label for="policy" class="text-[10px] font-black uppercase text-slate-400 ml-1">Update Strategy</label>
                                    <select 
                                        id="policy"
                                        bind:value={detail.rules.updatePolicy}
                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all font-bold appearance-none cursor-pointer"
                                    >
                                        <option value="auto">Automatic (Apply when available)</option>
                                        <option value="manual">Manual (Notify only)</option>
                                        <option value="locked">Locked (Ignore updates)</option>
                                    </select>
                                </div>

                                <div class="space-y-2">
                                    <label for="val-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Validation URL</label>
                                    <input 
                                        id="val-url"
                                        bind:value={detail.rules.validateUrl}
                                        placeholder="http://localhost:8080/health"
                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all font-mono"
                                    />
                                    <p class="text-[10px] text-slate-500">
                                        Used during update validation. Leave empty and HarborWatch derives this from labels, healthcheck, ports, and settings pattern.
                                    </p>
                                </div>

                                <div class="space-y-2">
                                    <label for="val-mode" class="text-[10px] font-black uppercase text-slate-400 ml-1">Validation Mode</label>
                                    <select
                                        id="val-mode"
                                        bind:value={detail.rules.validateMode}
                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all font-bold appearance-none cursor-pointer"
                                    >
                                        <option value="both">Both (HTTP + Docker)</option>
                                        <option value="docker">Docker Health Only</option>
                                        <option value="http">HTTP URL Only</option>
                                    </select>
                                </div>

                                <div class="grid grid-cols-2 gap-4">
                                    <div class="space-y-2">
                                        <label for="val-timeout" class="text-[10px] font-black uppercase text-slate-400 ml-1">Timeout (sec)</label>
                                        <input
                                            id="val-timeout"
                                            type="number"
                                            min="5"
                                            max="600"
                                            bind:value={detail.rules.validateTimeoutSec}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all font-mono"
                                        />
                                    </div>
                                    <div class="space-y-2">
                                        <label for="val-interval" class="text-[10px] font-black uppercase text-slate-400 ml-1">Interval (sec)</label>
                                        <input
                                            id="val-interval"
                                            type="number"
                                            min="1"
                                            max="30"
                                            bind:value={detail.rules.validateIntervalSec}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all font-mono"
                                        />
                                    </div>
                                </div>

                                <div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-slate-100 dark:border-slate-800">
                                    <div>
                                        <span class="text-sm font-bold text-slate-700 dark:text-slate-300">Auto-Rollback</span>
                                        <p class="text-[10px] text-slate-500 mt-0.5">Revert to last stable image if validation fails.</p>
                                    </div>
                                    <button 
                                        onclick={() => detail!.rules!.autoRollback = !detail!.rules!.autoRollback}
                                        class="w-10 h-5 rounded-full relative transition-colors {detail.rules.autoRollback ? 'bg-emerald-500' : 'bg-slate-300'}"
                                        aria-label="Toggle Auto-Rollback"
                                    >
                                        <div class="absolute top-1 w-3 h-3 bg-white rounded-full transition-all {detail.rules.autoRollback ? 'right-1' : 'left-1'}"></div>
                                    </button>
                                </div>

                                <div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-slate-100 dark:border-slate-800">
                                    <div>
                                        <span class="text-sm font-bold text-slate-700 dark:text-slate-300">AI Log Health Assessment</span>
                                        <p class="text-[10px] text-slate-500 mt-0.5">After validation, send recent container logs to AI and fail update if unhealthy.</p>
                                    </div>
                                    <button
                                        onclick={() => detail!.rules!.aiValidateLogs = !detail!.rules!.aiValidateLogs}
                                        class="w-10 h-5 rounded-full relative transition-colors {detail.rules.aiValidateLogs ? 'bg-brand-500' : 'bg-slate-300'}"
                                        aria-label="Toggle AI Log Health Assessment"
                                    >
                                        <div class="absolute top-1 w-3 h-3 bg-white rounded-full transition-all {detail.rules.aiValidateLogs ? 'right-1' : 'left-1'}"></div>
                                    </button>
                                </div>

                                <div class="space-y-3 p-4 bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-slate-100 dark:border-slate-800">
                                    <div class="flex items-center justify-between">
                                        <div>
                                            <span class="text-sm font-bold text-slate-700 dark:text-slate-300">Inherit Global Automation</span>
                                            <p class="text-[10px] text-slate-500 mt-0.5">Use global automation settings from Settings -> Automations.</p>
                                        </div>
                                        <button
                                            onclick={() => detail!.rules!.inheritAutomation = !detail!.rules!.inheritAutomation}
                                            class="w-10 h-5 rounded-full relative transition-colors {detail.rules.inheritAutomation ? 'bg-brand-500' : 'bg-slate-300'}"
                                            aria-label="Toggle Inherit Global Automation"
                                        >
                                            <div class="absolute top-1 w-3 h-3 bg-white rounded-full transition-all {detail.rules.inheritAutomation ? 'right-1' : 'left-1'}"></div>
                                        </button>
                                    </div>

                                    <div class="grid grid-cols-1 md:grid-cols-3 gap-2">
                                        <button
                                            onclick={() => detail!.rules!.upgradesAutomation = !detail!.rules!.upgradesAutomation}
                                            disabled={detail.rules.inheritAutomation}
                                            class="px-3 py-2 rounded-xl border text-[10px] font-black uppercase tracking-widest transition-all disabled:opacity-50 disabled:cursor-not-allowed {detail.rules.upgradesAutomation ? 'border-sky-300 text-sky-700 bg-sky-50 dark:bg-sky-900/20 dark:text-sky-300 dark:border-sky-900/40' : 'border-slate-300 text-slate-500 bg-white dark:bg-slate-900/30 dark:border-slate-700'}"
                                        >Upgrades</button>
                                        <button
                                            onclick={() => detail!.rules!.maintenanceAutomation = !detail!.rules!.maintenanceAutomation}
                                            disabled={detail.rules.inheritAutomation}
                                            class="px-3 py-2 rounded-xl border text-[10px] font-black uppercase tracking-widest transition-all disabled:opacity-50 disabled:cursor-not-allowed {detail.rules.maintenanceAutomation ? 'border-teal-300 text-teal-700 bg-teal-50 dark:bg-teal-900/20 dark:text-teal-300 dark:border-teal-900/40' : 'border-slate-300 text-slate-500 bg-white dark:bg-slate-900/30 dark:border-slate-700'}"
                                        >Maintenance</button>
                                        <button
                                            onclick={() => detail!.rules!.securityAutomation = !detail!.rules!.securityAutomation}
                                            disabled={detail.rules.inheritAutomation}
                                            class="px-3 py-2 rounded-xl border text-[10px] font-black uppercase tracking-widest transition-all disabled:opacity-50 disabled:cursor-not-allowed {detail.rules.securityAutomation ? 'border-orange-300 text-orange-700 bg-orange-50 dark:bg-orange-900/20 dark:text-orange-300 dark:border-orange-900/40' : 'border-slate-300 text-slate-500 bg-white dark:bg-slate-900/30 dark:border-slate-700'}"
                                        >Security</button>
                                    </div>
                                </div>

                                <button 
                                    onclick={saveRules}
                                    disabled={savingRules}
                                    class="w-full py-4 bg-brand-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20 hover:bg-brand-700 transition-all disabled:opacity-50"
                                >
                                    {savingRules ? 'Saving...' : 'Apply Policy Overrides'}
                                </button>
                                {#if lifecycleMessage}
                                    <p class="text-[10px] text-slate-500">{lifecycleMessage}</p>
                                {/if}
                            </div>
                        {/if}
                    </div>

                    <div class="space-y-6">
                        <div class="bg-white dark:bg-slate-800 p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                            <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider mb-6 flex items-center gap-2">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                                </svg>
                                Ad-hoc Actions
                            </h3>
                            <div class="space-y-3">
                                <button onclick={openManualUpdate} class="w-full py-4 bg-emerald-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-emerald-500/20 hover:bg-emerald-700 transition-all flex items-center justify-center gap-3">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                    </svg>
                                    Execute Manual Update
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            {:else if activeTab === 'configuration'}
                <div class="space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="bg-slate-900 rounded-3xl border border-slate-800 shadow-2xl overflow-hidden">
                        <div class="p-4 border-b border-slate-800 flex justify-between items-center bg-slate-800/50">
                            <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">Effective Compose Config</span>
                            <button 
                                onclick={runAudit}
                                disabled={auditing}
                                class="px-4 py-1.5 bg-brand-600 text-white rounded-lg font-black uppercase text-[9px] tracking-widest hover:bg-brand-700 transition-all disabled:opacity-50"
                            >
                                {auditing ? 'Analyzing...' : 'Audit with AI'}
                            </button>
                        </div>
                        <pre class="p-8 text-emerald-500 font-mono text-xs overflow-x-auto leading-relaxed"><code>{configYaml || '# Automated discovery pending. Click "Audit with AI" to retrieve and analyze.'}</code></pre>
                    </div>

                    {#if aiAudit}
                        <div class="bg-white dark:bg-slate-800 p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                            <h4 class="text-sm font-black uppercase text-slate-400 tracking-widest mb-6 flex items-center gap-2">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.67.335a2 2 0 01-1.797 0l-.67-.335a6 6 0 00-3.86-.517l-2.387.477a2 2 0 00-1.022.547l-1.162 1.162a2 2 0 00.597 3.301l1.557.519a8.001 8.001 0 0011.965 0l1.557-.519a2 2 0 00.597-3.301l-1.162-1.162z" />
                                </svg>
                                Compose Doctor Analysis
                            </h4>
                            <div class="prose dark:prose-invert prose-sm max-w-none text-slate-600 dark:text-slate-300 whitespace-pre-wrap leading-relaxed">
                                {aiAudit}
                            </div>
                        </div>
                    {/if}
                </div>
            {/if}
        </div>

        <!-- Action History -->
        <div class="mt-12 animate-in fade-in slide-in-from-bottom-4 duration-500 delay-150">
            <h3 class="text-sm font-black uppercase tracking-widest text-slate-400 mb-6 ml-2">Execution History</h3>
            <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
                <table class="w-full text-left text-sm">
                    <thead>
                        <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-400 text-[10px] font-black uppercase tracking-widest">
                            <th class="px-8 py-4">Action</th>
                            <th class="px-8 py-4">Target</th>
                            <th class="px-8 py-4">Status</th>
                            <th class="px-8 py-4">Time</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                        {#each detail.actionHistory || [] as action}
                            <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors">
                                <td class="px-8 py-4 font-bold text-slate-900 dark:text-white">{action.type}</td>
                                <td class="px-8 py-4 text-slate-500 font-mono text-xs">{action.target}</td>
                                <td class="px-8 py-4">
                                    <span class="px-2 py-1 rounded-lg font-bold text-[10px] uppercase {action.status === 'completed' ? 'bg-emerald-100 text-emerald-700' : 'bg-rose-100 text-rose-700'}">
                                        {action.status}
                                    </span>
                                </td>
                                <td class="px-8 py-4 text-slate-400">{new Date(action.startedAt * 1000).toLocaleString()}</td>
                            </tr>
                        {:else}
                            <tr>
                                <td colspan="4" class="px-8 py-12 text-center text-slate-400 italic">No historical actions found for this asset.</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        </div>
    {/if}
</div>
