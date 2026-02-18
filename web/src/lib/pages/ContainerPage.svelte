<script lang="ts">
    import { onMount } from "svelte";
    import type {
        ComposeAuditRecord,
        ComposeAuditRecordSummary,
        ContainerDetail,
        MalwareScanDetail,
        TrivyScanDetails,
        UpdateJobStatus
    } from "../api-types";
    import MetricChart from "../components/MetricChart.svelte";
    import DiskUsagePanel from "../components/DiskUsagePanel.svelte";
    import MalwareScanPanel from "../components/MalwareScanPanel.svelte";
    import TrivyFindingsPanel from "../components/TrivyFindingsPanel.svelte";
    import { toasts } from "../stores/ToastStore";

    interface ContainerIntelRecord {
        containerId: string;
        containerName?: string;
        image?: string;
        overrideRepositoryUrl?: string;
        overrideChangelogUrl?: string;
        derivedRepositoryUrl?: string;
        derivedChangelogUrl?: string;
        effectiveRepositoryUrl?: string;
        effectiveChangelogUrl?: string;
        updatedAt?: number;
    }

    let { id, onNavigate } = $props<{
        id: string;
        onNavigate: (route: string, params?: any) => void;
    }>();

    let detail = $state<ContainerDetail | null>(null);
    let configYaml = $state("");
    let aiAuditMarkdown = $state("");
    let aiAuditHtml = $state("");
    let composeAuditHistory = $state<ComposeAuditRecordSummary[]>([]);
    let loadingComposeAuditHistory = $state(false);
    let selectedComposeAuditId = $state("");
    let loading = $state(true);
    let auditing = $state(false);
    let savingRules = $state(false);
    let runningTrivyScan = $state(false);
    let stoppingTrivyScan = $state(false);
    let activeTrivyJobId = $state("");
    let runningMalwareScan = $state(false);
    let activeTab = $state("insights");
    let error = $state("");
    let scanMessage = $state("");
    let lifecycleMessage = $state("");
    let lifecycleMode = $state<"global" | "manual">("global");
    let updateHistory = $state<UpdateJobStatus[]>([]);
    let loadingUpdateHistory = $state(false);
    let vulnerabilityDetails = $state<TrivyScanDetails | null>(null);
    let loadingVulnerabilityDetails = $state(false);
    let malwareDetails = $state<MalwareScanDetail[]>([]);
    let loadingMalwareDetails = $state(false);
    let intel = $state<ContainerIntelRecord | null>(null);
    let loadingIntel = $state(false);
    let savingIntel = $state(false);

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
                syncLifecycleModeFromRules();
                await Promise.all([
                    loadVulnerabilityDetails(data?.summary?.image || ""),
                    loadMalwareDetailsForContainer(),
                    loadUpdateHistory(),
                    loadContainerIntel(),
                    loadComposeAuditHistory()
                ]);
            } else {
                const body = await res.json().catch(() => ({}));
                if (!silent) {
                    error = body?.message || `Container lookup failed (${res.status})`;
                }
                vulnerabilityDetails = null;
                malwareDetails = [];
                updateHistory = [];
                intel = null;
                composeAuditHistory = [];
                selectedComposeAuditId = "";
                aiAuditMarkdown = "";
                aiAuditHtml = "";
                configYaml = "";
            }
        } catch (e) {
            if (!silent) {
                error = "Failed to load container data";
            }
            vulnerabilityDetails = null;
            malwareDetails = [];
            updateHistory = [];
            intel = null;
            composeAuditHistory = [];
            selectedComposeAuditId = "";
            aiAuditMarkdown = "";
            aiAuditHtml = "";
            configYaml = "";
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

    async function loadUpdateHistory() {
        loadingUpdateHistory = true;
        try {
            const res = await fetch(`/api/updates/container/${id}?limit=20`);
            if (!res.ok) {
                updateHistory = [];
                return;
            }
            const rows = await res.json();
            updateHistory = Array.isArray(rows) ? rows : [];
        } catch {
            updateHistory = [];
        } finally {
            loadingUpdateHistory = false;
        }
    }

    async function loadContainerIntel() {
        loadingIntel = true;
        try {
            const res = await fetch(`/api/docker/${encodeURIComponent(id)}/intel`);
            if (!res.ok) {
                intel = null;
                return;
            }
            const payload = await res.json();
            intel = {
                ...payload,
                overrideRepositoryUrl: payload?.overrideRepositoryUrl || "",
                overrideChangelogUrl: payload?.overrideChangelogUrl || ""
            };
        } catch {
            intel = null;
        } finally {
            loadingIntel = false;
        }
    }

    async function saveContainerIntel() {
        if (!intel) return;
        savingIntel = true;
        try {
            const res = await fetch(`/api/docker/${encodeURIComponent(id)}/intel`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    repositoryUrl: intel.overrideRepositoryUrl || "",
                    changelogUrl: intel.overrideChangelogUrl || ""
                })
            });
            const body = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(body?.message || `Failed to save intelligence (${res.status})`);
            intel = {
                ...body,
                overrideRepositoryUrl: body?.overrideRepositoryUrl || "",
                overrideChangelogUrl: body?.overrideChangelogUrl || ""
            };
            toasts.success("Container intelligence saved.");
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to save container intelligence");
        } finally {
            savingIntel = false;
        }
    }

    function syncLifecycleModeFromRules() {
        lifecycleMode = detail?.rules?.inheritAutomation === false ? "manual" : "global";
    }

    function applyLifecycleModeToRules(mode: "global" | "manual") {
        if (!detail?.rules) return;
        lifecycleMode = mode;
        if (mode === "global") {
            detail.rules.inheritAutomation = true;
            detail.rules.upgradesAutomation = true;
            detail.rules.maintenanceAutomation = true;
            detail.rules.securityAutomation = true;
            return;
        }
        detail.rules.inheritAutomation = false;
        detail.rules.upgradesAutomation = false;
        detail.rules.maintenanceAutomation = false;
        detail.rules.securityAutomation = false;
        if (detail.rules.updatePolicy === "auto") {
            detail.rules.updatePolicy = "manual";
        }
    }

    function updatePolicyDescription(policy: string | undefined): string {
        const normalized = String(policy || "manual").toLowerCase();
        switch (normalized) {
            case "auto":
                return "Eligible for scheduled auto-apply when update automation is enabled.";
            case "locked":
                return "All upgrade runs are blocked for this container (manual and automated).";
            default:
                return "Updates are detected but applied only when manually triggered.";
        }
    }

    async function saveRules() {
        if (!detail?.rules) return;
        if (lifecycleMode === "manual" && detail.rules.updatePolicy === "auto") {
            detail.rules.updatePolicy = "manual";
        }
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
                lifecycleMessage = lifecycleMode === "global"
                    ? "Lifecycle mode set to global automation."
                    : "Lifecycle mode set to manual.";
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
        aiAuditMarkdown = "";
        aiAuditHtml = "";
        try {
            const res = await fetch(`/api/ai/audit-compose/${id}`);
            if (res.ok) {
                const data = await res.json();
                configYaml = data.config;
                aiAuditMarkdown = data.analysisMarkdown || data.analysis || "";
                aiAuditHtml = data.analysisHtml || "";
                selectedComposeAuditId = data.recordId || "";
                await loadComposeAuditHistory();
            } else {
                const body = await res.json().catch(() => ({}));
                toasts.error(body?.message || `Compose audit failed (${res.status})`);
            }
        } catch (e) {
            console.error("Audit failed", e);
            toasts.error("Compose audit failed.");
        } finally {
            auditing = false;
        }
    }

    async function loadComposeAuditHistory(selectFirst = true) {
        loadingComposeAuditHistory = true;
        try {
            const res = await fetch(`/api/ai/audit-compose/${id}/history?limit=30`);
            if (!res.ok) {
                composeAuditHistory = [];
                return;
            }
            const rows = await res.json();
            composeAuditHistory = Array.isArray(rows) ? rows : [];
            if (composeAuditHistory.length === 0) {
                selectedComposeAuditId = "";
                return;
            }
            if (selectedComposeAuditId && composeAuditHistory.some((row) => row.id === selectedComposeAuditId)) {
                return;
            }
            if (selectFirst) {
                await loadComposeAuditRecord(composeAuditHistory[0].id);
            }
        } catch {
            composeAuditHistory = [];
        } finally {
            loadingComposeAuditHistory = false;
        }
    }

    async function loadComposeAuditRecord(recordId: string) {
        const target = recordId?.trim();
        if (!target) return;
        try {
            const res = await fetch(`/api/ai/audit-compose/history/${encodeURIComponent(target)}`);
            if (!res.ok) return;
            const record: ComposeAuditRecord = await res.json();
            selectedComposeAuditId = record.id;
            configYaml = record.config || "";
            aiAuditMarkdown = record.analysisMarkdown || "";
            aiAuditHtml = record.analysisHtml || "";
        } catch {
            // Keep previous selected data if history fetch fails.
        }
    }

    async function triggerTrivyScan() {
        if (!detail || runningTrivyScan) return;
        runningTrivyScan = true;
        activeTrivyJobId = "";
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
            activeTrivyJobId = String(jobId);
            toasts.info("Trivy scan started.");
            scanMessage = "Trivy scan is running...";
            const done = await waitForScanJob(jobId);
            if (done === "completed") {
                toasts.success("Trivy scan completed.");
                scanMessage = "Trivy scan completed.";
                await loadDetail(true);
                return;
            }
            if (done === "cancelled") {
                toasts.info("Trivy scan cancelled.");
                scanMessage = "Trivy scan cancelled.";
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
            stoppingTrivyScan = false;
            activeTrivyJobId = "";
        }
    }

    async function waitForScanJob(jobId: string): Promise<"completed" | "failed" | "cancelled" | "running"> {
        const maxPolls = 40;
        for (let i = 0; i < maxPolls; i++) {
            await new Promise((resolve) => setTimeout(resolve, 1500));
            const res = await fetch(`/api/scans/jobs/${jobId}`);
            if (!res.ok) continue;
            const job = await res.json();
            const status = String(job?.status || "");
            if (status === "completed") return "completed";
            if (status === "failed") return "failed";
            if (status === "cancelled") return "cancelled";
        }
        return "running";
    }

    async function stopTrivyScan() {
        if (!activeTrivyJobId || !runningTrivyScan || stoppingTrivyScan) return;
        stoppingTrivyScan = true;
        try {
            const res = await fetch(`/api/scans/jobs/${encodeURIComponent(activeTrivyJobId)}/cancel`, {
                method: "POST"
            });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `cancel failed (${res.status})`);
            }
            scanMessage = "Trivy scan cancelled.";
            toasts.info("Trivy scan cancelled.");
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to stop Trivy scan.");
            stoppingTrivyScan = false;
        }
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
            {#each ['insights', 'security', 'lifecycle', 'intelligence', 'configuration'] as tab}
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
                            {#if runningTrivyScan && activeTrivyJobId}
                                <button
                                    onclick={stopTrivyScan}
                                    disabled={stoppingTrivyScan}
                                    class="w-full py-3 bg-rose-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-rose-500/20 hover:bg-rose-700 transition-all disabled:opacity-50"
                                >
                                    {stoppingTrivyScan ? 'Stopping...' : 'Stop Trivy Scan'}
                                </button>
                            {/if}
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
                <div class="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 items-start">
                        <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
                            <div>
                                <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Lifecycle Controls</h3>
                                <p class="text-[11px] text-slate-500 mt-1">Choose whether this container follows global automation policy or manual-only operation.</p>
                            </div>
                            {#if detail.rules}
                                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                                    <button
                                        onclick={() => applyLifecycleModeToRules("global")}
                                        class="px-4 py-3 rounded-2xl border text-[10px] font-black uppercase tracking-widest transition-colors {lifecycleMode === 'global' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                    >
                                        Use Global Automation
                                    </button>
                                    <button
                                        onclick={() => applyLifecycleModeToRules("manual")}
                                        class="px-4 py-3 rounded-2xl border text-[10px] font-black uppercase tracking-widest transition-colors {lifecycleMode === 'manual' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                    >
                                        Manual Mode
                                    </button>
                                </div>
                                <div class="space-y-2">
                                    <label for="update-policy" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Update Policy</label>
                                    <select
                                        id="update-policy"
                                        bind:value={detail.rules.updatePolicy}
                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                    >
                                        <option value="manual">Manual</option>
                                        <option value="auto" disabled={lifecycleMode === "manual"}>Auto Apply</option>
                                        <option value="locked">Locked</option>
                                    </select>
                                    <p class="text-[11px] text-slate-500">{updatePolicyDescription(detail.rules.updatePolicy)}</p>
                                    {#if lifecycleMode === "manual"}
                                        <p class="text-[11px] text-slate-500">Auto Apply is disabled while lifecycle mode is Manual.</p>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-3">
                                    <button
                                        onclick={saveRules}
                                        disabled={savingRules}
                                        class="px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest"
                                    >
                                        {savingRules ? "Saving..." : "Save Lifecycle Mode"}
                                    </button>
                                    <button
                                        onclick={openManualUpdate}
                                        disabled={detail.rules.updatePolicy === "locked"}
                                        class="px-4 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest"
                                    >
                                        Trigger Upgrade
                                    </button>
                                </div>
                                {#if lifecycleMessage}
                                    <p class="text-[11px] text-slate-500">{lifecycleMessage}</p>
                                {/if}
                            {/if}
                        </div>

                        <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                            <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Breaking Change Signals</h3>
                            <p class="text-[11px] text-slate-500 mt-1">Recent AI release assessments that flagged breaking-change risk.</p>
                            <div class="mt-4 space-y-3 max-h-[280px] overflow-y-auto pr-1">
                                {#if loadingUpdateHistory}
                                    <p class="text-[11px] text-slate-500">Loading lifecycle history...</p>
                                {:else}
                                    {#each updateHistory.filter((job) => (job.aiAnalysis?.breakingChanges || []).length > 0).slice(0, 6) as job}
                                        <div class="rounded-xl border border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-900/10 p-3">
                                            <p class="text-[10px] font-black uppercase tracking-widest text-amber-700 dark:text-amber-300">{job.targetImage}</p>
                                            <p class="text-[10px] text-amber-700/80 dark:text-amber-200/90 mt-1">{new Date(job.updatedAt * 1000).toLocaleString()}</p>
                                            <p class="text-[11px] text-amber-900 dark:text-amber-100 mt-2">{job.aiAnalysis?.breakingChanges?.[0]}</p>
                                        </div>
                                    {:else}
                                        <p class="text-[11px] text-slate-500 italic">No recent breaking-change notices for this container.</p>
                                    {/each}
                                {/if}
                            </div>
                        </div>
                    </div>

                    <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
                        <div class="px-6 py-4 border-b border-slate-100 dark:border-slate-700">
                            <h3 class="text-sm font-black uppercase tracking-widest text-slate-400">Lifecycle Log</h3>
                        </div>
                        <div class="overflow-x-auto">
                            <table class="w-full text-left text-sm">
                                <thead>
                                    <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-400 text-[10px] font-black uppercase tracking-widest">
                                        <th class="px-6 py-3">Target</th>
                                        <th class="px-6 py-3">Status</th>
                                        <th class="px-6 py-3">Risk</th>
                                        <th class="px-6 py-3">Updated</th>
                                    </tr>
                                </thead>
                                <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                                    {#if loadingUpdateHistory}
                                        <tr>
                                            <td colspan="4" class="px-6 py-8 text-center text-slate-500 italic">Loading lifecycle entries...</td>
                                        </tr>
                                    {:else}
                                        {#each updateHistory as job}
                                            <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors align-top">
                                                <td class="px-6 py-4 text-xs font-mono text-slate-600 dark:text-slate-300 break-all">{job.targetImage}</td>
                                                <td class="px-6 py-4">
                                                    <span class="px-2 py-1 rounded-lg font-bold text-[10px] uppercase {job.status === 'completed' ? 'bg-emerald-100 text-emerald-700' : job.status === 'failed' ? 'bg-rose-100 text-rose-700' : 'bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200'}">
                                                        {job.status}
                                                    </span>
                                                </td>
                                                <td class="px-6 py-4 text-[11px] text-slate-500">
                                                    {job.aiAnalysis ? `${job.aiAnalysis.riskLevel} (${job.aiAnalysis.riskScore})` : "n/a"}
                                                </td>
                                                <td class="px-6 py-4 text-[11px] text-slate-500">{new Date(job.updatedAt * 1000).toLocaleString()}</td>
                                            </tr>
                                        {:else}
                                            <tr>
                                                <td colspan="4" class="px-6 py-10 text-center text-slate-400 italic">No lifecycle events recorded yet for this container.</td>
                                            </tr>
                                        {/each}
                                    {/if}
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>
            {:else if activeTab === 'intelligence'}
                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
                        <div>
                            <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Repository Intelligence Overrides</h3>
                            <p class="text-[11px] text-slate-500 mt-1">Set repository/changelog overrides when container labels are missing or incorrect.</p>
                        </div>
                        {#if loadingIntel}
                            <p class="text-sm text-slate-500">Loading intelligence data...</p>
                        {:else if intel}
                            <div class="space-y-2">
                                <label for="intel-repo" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Repository URL Override</label>
                                <input
                                    id="intel-repo"
                                    type="url"
                                    bind:value={intel.overrideRepositoryUrl}
                                    placeholder="https://github.com/org/repo"
                                    class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                />
                            </div>
                            <div class="space-y-2">
                                <label for="intel-changelog" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Changelog URL Override</label>
                                <input
                                    id="intel-changelog"
                                    type="url"
                                    bind:value={intel.overrideChangelogUrl}
                                    placeholder="https://github.com/org/repo/releases"
                                    class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                />
                            </div>
                            <button
                                onclick={saveContainerIntel}
                                disabled={savingIntel}
                                class="px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest"
                            >
                                {savingIntel ? "Saving..." : "Save Intelligence"}
                            </button>
                        {:else}
                            <p class="text-sm text-slate-500">Intelligence endpoint unavailable.</p>
                        {/if}
                    </div>

                    <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
                        <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Effective Metadata</h3>
                        {#if intel}
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-3">
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Derived Repository</p>
                                <p class="text-xs text-slate-700 dark:text-slate-300 break-all mt-1">{intel.derivedRepositoryUrl || "None"}</p>
                            </div>
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-3">
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Derived Changelog</p>
                                <p class="text-xs text-slate-700 dark:text-slate-300 break-all mt-1">{intel.derivedChangelogUrl || "None"}</p>
                            </div>
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-brand-50 dark:bg-brand-900/20 p-3">
                                <p class="text-[10px] font-black uppercase tracking-widest text-brand-700 dark:text-brand-300">Effective Repository</p>
                                <p class="text-xs text-brand-800 dark:text-brand-200 break-all mt-1">{intel.effectiveRepositoryUrl || "None"}</p>
                            </div>
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-brand-50 dark:bg-brand-900/20 p-3">
                                <p class="text-[10px] font-black uppercase tracking-widest text-brand-700 dark:text-brand-300">Effective Changelog</p>
                                <p class="text-xs text-brand-800 dark:text-brand-200 break-all mt-1">{intel.effectiveChangelogUrl || "None"}</p>
                            </div>
                            <p class="text-[11px] text-slate-500">Last override update: {intel.updatedAt ? new Date(intel.updatedAt * 1000).toLocaleString() : "Never"}</p>
                        {:else}
                            <p class="text-sm text-slate-500">No metadata available.</p>
                        {/if}
                    </div>
                </div>
            {:else if activeTab === 'configuration'}
                <div class="space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                        <div class="flex items-center justify-between gap-3 mb-4">
                            <h4 class="text-sm font-black uppercase text-slate-400 tracking-widest">Compose Audit History</h4>
                            <button
                                onclick={() => loadComposeAuditHistory(false)}
                                disabled={loadingComposeAuditHistory}
                                class="px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 text-[9px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50"
                            >
                                {loadingComposeAuditHistory ? "Refreshing..." : "Refresh"}
                            </button>
                        </div>
                        <div class="space-y-2 max-h-[220px] overflow-y-auto pr-1">
                            {#if loadingComposeAuditHistory}
                                <p class="text-[11px] text-slate-500">Loading compose audit history...</p>
                            {:else}
                                {#each composeAuditHistory as record}
                                    <button
                                        onclick={() => loadComposeAuditRecord(record.id)}
                                        class="w-full text-left rounded-xl border px-3 py-2 transition-colors {selectedComposeAuditId === record.id ? 'border-brand-500 bg-brand-50 dark:bg-brand-900/20' : 'border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-900/30'}"
                                    >
                                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">
                                            {new Date(record.createdAt * 1000).toLocaleString()}
                                        </p>
                                        <p class="mt-1 text-xs font-semibold text-slate-800 dark:text-slate-200">
                                            {record.headline || "Compose audit"}
                                        </p>
                                        <p class="mt-1 text-[10px] text-slate-500">{record.provider} / {record.model}</p>
                                    </button>
                                {:else}
                                    <p class="text-[11px] text-slate-500 italic">No compose audits persisted yet. Run an audit to build history.</p>
                                {/each}
                            {/if}
                        </div>
                    </div>

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
                        <pre class="p-8 text-emerald-500 font-mono text-xs overflow-x-auto leading-relaxed"><code>{configYaml || '# Automated discovery pending. Click "Audit with AI" to retrieve, analyze, and persist.'}</code></pre>
                    </div>

                    {#if aiAuditMarkdown}
                        <div class="bg-white dark:bg-slate-800 p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm">
                            <h4 class="text-sm font-black uppercase text-slate-400 tracking-widest mb-6 flex items-center gap-2">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.67.335a2 2 0 01-1.797 0l-.67-.335a6 6 0 00-3.86-.517l-2.387.477a2 2 0 00-1.022.547l-1.162 1.162a2 2 0 00.597 3.301l1.557.519a8.001 8.001 0 0011.965 0l1.557-.519a2 2 0 00.597-3.301l-1.162-1.162z" />
                                </svg>
                                Compose Doctor Analysis
                            </h4>
                            {#if aiAuditHtml}
                                <div class="markdown-content text-sm text-slate-700 dark:text-slate-200 leading-relaxed">
                                    {@html aiAuditHtml}
                                </div>
                            {:else}
                                <div class="text-sm text-slate-600 dark:text-slate-300 whitespace-pre-wrap leading-relaxed">{aiAuditMarkdown}</div>
                            {/if}
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
