<script lang="ts">
    import { onMount } from "svelte";
    import type {
        ComposeAuditRecord,
        ComposeAuditRecordSummary,
        ContainerDetail,
        MalwareScanDetail,
        ScanJobStatus,
        TrivyScanDetails,
        UpdateJobStatus
    } from "../api-types";
    import MetricChart from "../components/MetricChart.svelte";
    import DiskUsagePanel from "../components/DiskUsagePanel.svelte";
    import MalwareScanPanel from "../components/MalwareScanPanel.svelte";
    import TrivyFindingsPanel from "../components/TrivyFindingsPanel.svelte";
    import PortainerLogo from "../components/PortainerLogo.svelte";
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
        repositoryProvider?: string;
        hasRepository?: boolean;
        hasChangelog?: boolean;
        releaseIntelReady?: boolean;
        fullAutomationReady?: boolean;
        portainerManaged?: boolean;
        portainerConfigured?: boolean;
        issues?: Array<{
            code: string;
            severity: "info" | "warning" | "error";
            message: string;
            action?: string;
        }>;
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
    let bypassAi = $state(false);
    let updateHistory = $state<UpdateJobStatus[]>([]);
    let loadingUpdateHistory = $state(false);
    let vulnerabilityDetails = $state<TrivyScanDetails | null>(null);
    let loadingVulnerabilityDetails = $state(false);
    let malwareDetails = $state<MalwareScanDetail[]>([]);
    let malwareJobs = $state<ScanJobStatus[]>([]);
    let loadingMalwareDetails = $state(false);
    let intel = $state<ContainerIntelRecord | null>(null);
    let loadingIntel = $state(false);
    let savingIntel = $state(false);

    function activeContainerId(): string {
        return String(detail?.summary?.id || id || "").trim();
    }

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
                detail = data;
                syncLifecycleModeFromRules();
                await Promise.all([
                    loadVulnerabilityDetails(data?.summary?.image || ""),
                    loadMalwareSummaryForContainer(),
                    loadMalwareDetailsForContainer(),
                    loadMalwareJobsForContainer(),
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
                malwareJobs = [];
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
            malwareJobs = [];
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
        const containerId = activeContainerId();
        if (!containerId) {
            malwareDetails = [];
            return;
        }
        loadingMalwareDetails = true;
        try {
            const prefix = `container:${containerId}`;
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

    async function loadMalwareSummaryForContainer() {
        const containerId = activeContainerId();
        if (!containerId) {
            if (detail) detail.malwareSummary = [];
            return;
        }
        try {
            const res = await fetch(`/api/scans/malware/container/${encodeURIComponent(containerId)}/summary`);
            if (!res.ok) {
                if (detail) detail.malwareSummary = [];
                return;
            }
            const rows = await res.json();
            if (detail) {
                detail.malwareSummary = Array.isArray(rows) ? rows : [];
            }
        } catch {
            if (detail) detail.malwareSummary = [];
        }
    }

    async function loadMalwareJobsForContainer() {
        const containerId = activeContainerId();
        if (!containerId) {
            malwareJobs = [];
            return;
        }
        try {
            const prefix = `container:${containerId}`;
            const res = await fetch(`/api/scans/jobs?type=malware&prefix=${encodeURIComponent(prefix)}&limit=20`);
            if (!res.ok) {
                malwareJobs = [];
                return;
            }
            const rows = await res.json();
            malwareJobs = Array.isArray(rows) ? rows : [];
        } catch {
            malwareJobs = [];
        }
    }

    async function loadUpdateHistory() {
        const containerId = activeContainerId();
        if (!containerId) {
            updateHistory = [];
            return;
        }
        loadingUpdateHistory = true;
        try {
            const res = await fetch(`/api/updates/container/${encodeURIComponent(containerId)}?limit=20`);
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
        const containerId = activeContainerId();
        if (!containerId) {
            intel = null;
            return;
        }
        loadingIntel = true;
        try {
            const res = await fetch(`/api/docker/${encodeURIComponent(containerId)}/intel`);
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
        const containerId = activeContainerId();
        if (!containerId) return;
        savingIntel = true;
        try {
            const res = await fetch(`/api/docker/${encodeURIComponent(containerId)}/intel`, {
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
            detail.rules.updatePolicy = "auto";
            return;
        }
        detail.rules.inheritAutomation = false;
        detail.rules.upgradesAutomation = false;
        detail.rules.maintenanceAutomation = false;
        detail.rules.securityAutomation = false;
        detail.rules.updatePolicy = "manual";
    }

    async function saveRules() {
        if (!detail?.rules) return;
        const containerId = activeContainerId();
        if (!containerId) return;
        
        lifecycleMessage = "";
        savingRules = true;
        try {
            const res = await fetch(`/api/docker/${encodeURIComponent(containerId)}/rules`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(detail.rules)
            });
            if (res.ok) {
                await loadDetail(true);
                lifecycleMessage = lifecycleMode === "global"
                    ? "Lifecycle mode set to Automatic (Follows global policy)."
                    : "Lifecycle mode set to Manual (User triggered only).";
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
        const containerId = activeContainerId();
        if (!containerId) return;
        auditing = true;
        aiAuditMarkdown = "";
        aiAuditHtml = "";
        try {
            const res = await fetch(`/api/ai/audit-compose/${encodeURIComponent(containerId)}`);
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
        const containerId = activeContainerId();
        if (!containerId) {
            composeAuditHistory = [];
            selectedComposeAuditId = "";
            return;
        }
        loadingComposeAuditHistory = true;
        try {
            const res = await fetch(`/api/ai/audit-compose/${encodeURIComponent(containerId)}/history?limit=30`);
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
        const containerId = activeContainerId();
        if (!containerId) return;
        runningMalwareScan = true;
        scanMessage = "Queueing ClamAV container scan...";
        const previousTop = detail?.malwareSummary?.[0]?.scannedAt || 0;
        const scanStartTs = Math.floor(Date.now() / 1000);
        try {
            const res = await fetch(`/api/scans/malware/container/${encodeURIComponent(containerId)}`, { method: "POST" });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `scan failed (${res.status})`);
            }
            toasts.success("ClamAV scans queued for container rootfs and mounts.");
            scanMessage = "ClamAV scan queued. Waiting for results...";
            await refreshMalwareHistory(previousTop, scanStartTs);
        } catch (e) {
            scanMessage = "Failed to queue ClamAV scan.";
            toasts.error(e instanceof Error ? e.message : "Failed to queue ClamAV scan.");
        } finally {
            runningMalwareScan = false;
        }
    }

    function summarizeMalwareJobs(scanStartTs: number): string {
        const recentJobs = malwareJobs.filter((job) => Number(job.startedAt || 0) >= scanStartTs - 2);
        if (recentJobs.length === 0) return "";
        let queued = 0;
        let running = 0;
        let failed = 0;
        let completed = 0;
        for (const job of recentJobs) {
            const status = String(job.status || "").toLowerCase();
            if (status === "queued") queued += 1;
            else if (status === "running") running += 1;
            else if (status === "completed") completed += 1;
            else if (status === "failed" || status === "cancelled") failed += 1;
        }
        return `Jobs: queued ${queued}, running ${running}, completed ${completed}, failed ${failed}`;
    }

    async function refreshMalwareHistory(previousTopScannedAt: number, scanStartTs: number) {
        for (let i = 0; i < 30; i++) {
            await new Promise((resolve) => setTimeout(resolve, 3000));
            await Promise.all([
                loadMalwareSummaryForContainer(),
                loadMalwareDetailsForContainer(),
                loadMalwareJobsForContainer()
            ]);
            const top = detail?.malwareSummary?.[0]?.scannedAt || 0;
            if (top > previousTopScannedAt) {
                scanMessage = "ClamAV results updated.";
                toasts.success("ClamAV scan finished and results were added.");
                return;
            }
            const jobSummary = summarizeMalwareJobs(scanStartTs);
            scanMessage = jobSummary ? `ClamAV scan queued. ${jobSummary}` : "ClamAV scan queued. Waiting for scanner workers...";
        }
        scanMessage = "ClamAV scan is still running in background.";
        toasts.info("ClamAV scan is still running. Results will appear when complete.");
    }

    function openManualUpdate() {
        if (!detail) return;
        onNavigate("updates", {
            containerId: detail.summary.id,
            targetImage: detail.summary.image,
            validateUrl: detail.rules?.validateUrl || "",
            validateMode: detail.rules?.validateMode || "both",
            validateTimeoutSec: detail.rules?.validateTimeoutSec || 45,
            validateIntervalSec: detail.rules?.validateIntervalSec || 2,
            bypassAi: bypassAi
        });
    }

    function intelStatusClass(): string {
        if (!intel) return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
        if (intel.fullAutomationReady) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
    }

    function intelStatusLabel(): string {
        if (!intel) return "Unknown";
        if (intel.fullAutomationReady) return "Automation Ready";
        if (intel.releaseIntelReady) return "Partial Readiness";
        return "Action Needed";
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

    const healthColor = (health: string | undefined) => {
        switch (health?.toLowerCase()) {
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
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-6 bg-white dark:bg-slate-800 p-6 md:p-8 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl shadow-slate-200/50 dark:shadow-none overflow-hidden">
            <div class="flex items-center gap-4 md:gap-6 min-w-0">
                <div class="relative flex-shrink-0">
                    <div class="w-16 h-16 md:w-20 md:h-20 rounded-2xl bg-brand-600 flex items-center justify-center text-white shadow-lg shadow-brand-500/20">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 md:h-10 md:w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                        </svg>
                    </div>
                    <div class="absolute -bottom-1 -right-1 w-5 h-5 md:w-6 md:h-6 rounded-full border-4 border-white dark:border-slate-800 {stateColor(detail.summary.state)}"></div>
                </div>
                <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                        <h2 class="text-xl md:text-3xl font-black text-slate-900 dark:text-white tracking-tight truncate max-w-full">
                            {detail.summary.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                        </h2>
                        {#if detail.summary.health && detail.summary.health !== "none"}
                             <span class="px-2 py-1 bg-slate-100 dark:bg-slate-900/50 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-slate-700 rounded text-[9px] font-black uppercase flex items-center gap-1.5">
                                <span class="w-1.5 h-1.5 rounded-full {healthColor(detail.summary.health)} animate-pulse"></span>
                                {detail.summary.health}
                            </span>
                        {/if}
                        {#if detail.summary.updateAvailable}
                            <span class="px-2 py-1 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded text-[9px] font-black uppercase animate-pulse whitespace-nowrap">
                                Update Available
                            </span>
                        {/if}
                        {#if intel?.portainerManaged}
                            <span class="px-2 py-1 {intel?.portainerConfigured ? 'bg-cyan-500/10 text-cyan-500 badge-cyan-glow' : 'bg-amber-500 text-white'} rounded-md text-[9px] font-black uppercase whitespace-nowrap flex items-center gap-1.5">
                                <PortainerLogo size={12} />
                                Portainer Managed
                            </span>
                        {/if}
                    </div>
                    <p class="text-slate-500 font-mono text-[10px] md:text-sm mt-1 truncate max-w-full">{detail.summary.image}</p>
                </div>
            </div>

            <div class="flex gap-2 w-full md:w-auto">
                <button class="flex-1 md:flex-none px-4 md:px-6 py-3 bg-slate-900 dark:bg-white text-white dark:text-slate-900 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg hover:scale-105 transition-all">
                    Restart
                </button>
                <button class="flex-1 md:flex-none px-4 md:px-6 py-3 bg-rose-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-rose-500/20 hover:scale-105 transition-all">
                    Stop
                </button>
            </div>
        </div>

        <!-- Navigation Tabs -->
        <div class="flex flex-wrap gap-1 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl w-full md:w-fit border border-slate-200 dark:border-slate-800">
            {#each ['insights', 'security', 'lifecycle', 'intelligence', 'configuration'] as tab}
                <button 
                    onclick={() => activeTab = tab}
                    class="flex-1 md:flex-none px-3 md:px-6 py-2.5 rounded-xl text-[9px] md:text-[10px] font-black uppercase tracking-widest transition-all {activeTab === tab ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
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

                    <!-- Moved Execution History here -->
                    <div class="lg:col-span-2">
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
                    {#if intel?.portainerManaged && !intel?.portainerConfigured}
                        <div class="bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-800/50 p-4 rounded-2xl flex items-start gap-3">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                            </svg>
                            <div class="text-xs">
                                <p class="font-black text-amber-900 dark:text-amber-200 uppercase tracking-tight">Portainer Integration Required</p>
                                <p class="text-amber-800/80 dark:text-amber-300/70 mt-1">
                                    This container is managed by Portainer. HarborWatch requires Portainer API access to safely update this stack and preserve its configuration and secrets.
                                </p>
                                <button 
                                    onclick={() => onNavigate('settings')}
                                    class="mt-2 text-amber-700 dark:text-amber-400 font-black uppercase hover:underline"
                                >Configure Portainer in Settings &rarr;</button>
                            </div>
                        </div>
                    {/if}

                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 items-start">
                        <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
                            <div>
                                <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Lifecycle Management</h3>
                                <p class="text-[11px] text-slate-500 mt-1">Choose whether this container updates automatically or requires manual intervention.</p>
                            </div>
                            {#if detail.rules}
                                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                                    <button
                                        onclick={() => applyLifecycleModeToRules("global")}
                                        class="px-4 py-3 rounded-2xl border text-[10px] font-black uppercase tracking-widest transition-colors {lifecycleMode === 'global' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-500'}"
                                    >
                                        Automatic
                                    </button>
                                    <button
                                        onclick={() => applyLifecycleModeToRules("manual")}
                                        class="px-4 py-3 rounded-2xl border text-[10px] font-black uppercase tracking-widest transition-colors {lifecycleMode === 'manual' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-500'}"
                                    >
                                        Manual
                                    </button>
                                </div>

                                <div class="p-4 rounded-2xl bg-slate-50 dark:bg-slate-900/30 border border-slate-100 dark:border-slate-700 space-y-2">
                                    {#if lifecycleMode === "global"}
                                        <p class="text-[11px] text-slate-600 dark:text-slate-300 leading-relaxed font-medium">
                                            Follows global automation settings. Updates will be applied automatically if the "Container Auto-Apply" schedule is enabled.
                                        </p>
                                    {:else}
                                        <div class="space-y-3">
                                            <p class="text-[11px] text-slate-600 dark:text-slate-300 leading-relaxed font-medium">
                                                Automation is paused. You must explicitly trigger the upgrade flow using the button below.
                                            </p>
                                            <div class="flex items-center justify-between pt-2 border-t border-slate-200/50 dark:border-slate-700/50">
                                                <div>
                                                    <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Bypass AI Assessment</p>
                                                    <p class="text-[11px] text-slate-500">Skip AI breaking-change analysis for manual runs.</p>
                                                </div>
                                                <button
                                                    onclick={() => bypassAi = !bypassAi}
                                                    class="w-10 h-5 rounded-full transition-colors relative {bypassAi ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                    aria-label="Toggle AI Bypass"
                                                >
                                                    <div class="absolute top-1 left-1 w-3 h-3 rounded-full bg-white transition-transform {bypassAi ? 'translate-x-5' : ''}"></div>
                                                </button>
                                            </div>
                                        </div>
                                    {/if}
                                </div>

                                <div class="flex flex-wrap gap-3 pt-2">
                                    <button
                                        onclick={saveRules}
                                        disabled={savingRules}
                                        class="px-5 py-2.5 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 text-[10px] font-black uppercase tracking-widest hover:scale-105 transition-all disabled:opacity-60"
                                    >
                                        {savingRules ? "Saving..." : "Save Selection"}
                                    </button>
                                    <button
                                        onclick={openManualUpdate}
                                        disabled={intel?.portainerManaged && !intel?.portainerConfigured}
                                        class="px-5 py-2.5 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest shadow-lg shadow-brand-500/20 hover:scale-105 transition-all"
                                        title={intel?.portainerManaged && !intel?.portainerConfigured ? "Portainer integration required" : ""}
                                    >
                                        Trigger Upgrade
                                    </button>
                                </div>
                                {#if lifecycleMessage}
                                    <p class="text-[11px] text-emerald-600 font-bold uppercase tracking-tighter">{lifecycleMessage}</p>
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
                        <div class="flex items-center justify-between gap-3">
                            <h3 class="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Effective Metadata</h3>
                            <span class="px-2 py-1 rounded-lg text-[10px] font-black uppercase {intelStatusClass()}">{intelStatusLabel()}</span>
                        </div>
                        {#if intel}
                            {#if intel.issues && intel.issues.length > 0}
                                <div class="rounded-xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-900/10 p-3 space-y-2">
                                    <p class="text-[10px] font-black uppercase tracking-widest text-rose-700 dark:text-rose-300">Intelligence Issues</p>
                                    {#each intel.issues as issue}
                                        <div class="border border-rose-200/80 dark:border-rose-900/50 rounded-lg px-2.5 py-2 bg-white/70 dark:bg-slate-900/20">
                                            <p class="text-[11px] font-semibold text-rose-800 dark:text-rose-200">{issue.message}</p>
                                            {#if issue.action}
                                                <p class="mt-1 text-[10px] text-rose-700/90 dark:text-rose-200/90">{issue.action}</p>
                                            {/if}
                                        </div>
                                    {/each}
                                </div>
                            {/if}
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
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-3">
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Release Provider</p>
                                <p class="text-xs text-slate-700 dark:text-slate-300 break-all mt-1">{intel.repositoryProvider || "unknown"}</p>
                            </div>
                            
                            <div class="rounded-xl border border-brand-200 dark:border-brand-900/40 bg-brand-50 dark:bg-brand-900/10 p-4 space-y-2">
                                <p class="text-[10px] font-black uppercase tracking-widest text-brand-700 dark:text-brand-300">Pro-Tip: Optimal Setup</p>
                                <p class="text-[11px] text-brand-800/80 dark:text-brand-200/80 leading-relaxed">
                                    To maximize AI accuracy, ensure your container includes metadata. HarborWatch looks for:
                                </p>
                                <ul class="list-disc list-inside text-[10px] text-brand-700/70 dark:text-brand-300/70 space-y-1">
                                    <li><code class="bg-brand-100 dark:bg-brand-900/40 px-1 rounded">org.opencontainers.image.source</code> label</li>
                                    <li><code class="bg-brand-100 dark:bg-brand-900/40 px-1 rounded">harborwatch.intel.url</code> override label</li>
                                    <li>Explicit overrides set in this tab</li>
                                </ul>
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
    {/if}
</div>
