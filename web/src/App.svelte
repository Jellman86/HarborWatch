<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import Dashboard from "./lib/pages/Dashboard.svelte";
  import Containers from "./lib/pages/Containers.svelte";
  import Images from "./lib/pages/Images.svelte";
  import Security from "./lib/pages/Security.svelte";
  import Intelligence from "./lib/pages/Intelligence.svelte";
  import Updates from "./lib/pages/Updates.svelte";
  import AuditLog from "./lib/pages/AuditLog.svelte";
  import Settings from "./lib/pages/Settings.svelte";
  import { layoutStore } from "./lib/stores/layout.svelte";
  import { themeStore } from "./lib/stores/theme.svelte";
  
  import type {
    ContainerSummary,
    DockerEvent,
    HealthResponse,
    ImageSummary,
    ReleaseRiskSummary,
    ScanJobStatus,
    ScanStartResponse,
    ScanSummary,
    MalwareScanSummary,
    UpdateJobStatus,
    UpdateStartResponse,
    UpdateStepEvent
  } from "./lib/api-types";

  // Navigation State
  let currentRoute = $state("dashboard");

  // Global Data State
  let health = $state<HealthResponse | null>(null);
  let containers = $state<ContainerSummary[]>([]);
  let images = $state<ImageSummary[]>([]);
  let events = $state<DockerEvent[]>([]);
  let summary = $state<ScanSummary | null>(null);
  let malwareSummaries = $state<MalwareScanSummary[]>([]);
  let releaseSummary = $state<ReleaseRiskSummary | null>(null);
  let activeJob = $state<ScanJobStatus | null>(null);
  let updateJob = $state<UpdateJobStatus | null>(null);
  let updateLive = $state<UpdateStepEvent[]>([]);

  // Form State (Default Values)
  let target = $state("nginx:latest");
  let malwareTarget = $state("/var/lib/docker");
  let repo = $state("Jellman86/HarborWatch");
  let updateContainerId = $state("");
  let updateTargetImage = $state("ghcr.io/jellman86/harborwatch:latest");
  let validateURL = $state("http://localhost:18080/health");

  // Error State
  let error = $state("");
  let scanError = $state("");
  let releaseError = $state("");
  let updateError = $state("");

  let eventSource: EventSource | null = null;
  let updateEventSource: EventSource | null = null;
  let pollTimer: number | null = null;
  let updatePollTimer: number | null = null;

  async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
    const response = await fetch(url, init);
    if (!response.ok) throw new Error(`${url} failed (${response.status}): ${await response.text()}`);
    return (await response.json()) as T;
  }

  function connectEvents() {
    eventSource?.close();
    eventSource = new EventSource("/api/docker/events");
    eventSource.addEventListener("docker", (evt) => {
      try {
        const parsed = JSON.parse((evt as MessageEvent).data) as DockerEvent;
        events = [parsed, ...events].slice(0, 50);
      } catch {}
    });
  }

  function connectUpdateEvents(jobId: string) {
    updateEventSource?.close();
    updateEventSource = new EventSource(`/api/updates/events/${jobId}`);
    updateEventSource.addEventListener("update", (evt) => {
      try {
        const parsed = JSON.parse((evt as MessageEvent).data) as UpdateStepEvent;
        updateLive = [...updateLive, parsed].slice(-100);
      } catch {}
    });
  }

  async function loadSummary() {
    try { 
      summary = await fetchJSON<ScanSummary>("/api/scans/summary"); 
      scanError = ""; 
    } catch (e) { 
      if (!(e instanceof Error && e.message.includes("(404)"))) scanError = e instanceof Error ? e.message : "Unknown error"; 
    }
  }

  async function loadMalwareSummaries() {
    try {
      malwareSummaries = await fetchJSON<MalwareScanSummary[]>("/api/scans/malware/summary");
    } catch (e) {
      console.error("Failed to load malware summaries", e);
    }
  }

  async function loadReleaseSummary() {
    try { 
      releaseSummary = await fetchJSON<ReleaseRiskSummary>(`/api/releases/summary?repo=${encodeURIComponent(repo)}`); 
      releaseError = ""; 
    } catch (e) { 
      releaseError = e instanceof Error ? e.message : "Release summary failed"; 
    }
  }

  async function load() {
    error = "";
    try {
      const [h, c, i] = await Promise.all([
        fetchJSON<HealthResponse>("/health"),
        fetchJSON<ContainerSummary[]>("/api/docker/containers"),
        fetchJSON<ImageSummary[]>("/api/docker/images")
      ]);
      health = h; containers = c; images = i; 
      if (containers.length > 0 && !updateContainerId) updateContainerId = containers[0].id;
      connectEvents(); 
      await loadSummary(); 
      await loadMalwareSummaries();
      await loadReleaseSummary();
    } catch (e) { error = e instanceof Error ? e.message : "Unknown error"; }
  }

  async function pollJob(jobId: string) {
    if (pollTimer) window.clearInterval(pollTimer);
    pollTimer = window.setInterval(async () => {
      try {
        const job = await fetchJSON<ScanJobStatus>(`/api/scans/jobs/${jobId}`);
        activeJob = job;
        if (job.status === "completed" || job.status === "failed") { 
          if (pollTimer) window.clearInterval(pollTimer); 
          pollTimer = null; 
          await loadSummary(); 
          await loadMalwareSummaries();
        }
      } catch (e) { scanError = e instanceof Error ? e.message : "Failed to poll scan job"; }
    }, 2000);
  }

  async function startScan() {
    scanError = "";
    try {
      const response = await fetchJSON<ScanStartResponse>("/api/scans/run", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ target }) });
      activeJob = { jobId: response.jobId, target, status: response.status, source: "trivy", startedAt: Math.floor(Date.now() / 1000) };
      await pollJob(response.jobId);
    } catch (e) { scanError = e instanceof Error ? e.message : "Scan request failed"; }
  }

  async function startMalwareScan() {
    scanError = "";
    try {
      const response = await fetchJSON<ScanStartResponse>("/api/scans/malware/run", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ target: malwareTarget }) });
      activeJob = { jobId: response.jobId, target: malwareTarget, status: response.status, source: "clamav", startedAt: Math.floor(Date.now() / 1000) };
      await pollJob(response.jobId);
    } catch (e) { scanError = e instanceof Error ? e.message : "Malware scan failed"; }
  }

  async function pollUpdate(jobId: string) {
    if (updatePollTimer) window.clearInterval(updatePollTimer);
    updatePollTimer = window.setInterval(async () => {
      try {
        const job = await fetchJSON<UpdateJobStatus>(`/api/updates/jobs/${jobId}`);
        updateJob = job;
        if (job.status === "completed" || job.status === "failed" || job.status === "rolled_back") {
          if (updatePollTimer) window.clearInterval(updatePollTimer);
          updatePollTimer = null;
        }
      } catch (e) { updateError = e instanceof Error ? e.message : "Failed to poll update job"; }
    }, 1500);
  }

  async function startUpdate() {
    updateError = "";
    updateLive = [];
    try {
      const response = await fetchJSON<UpdateStartResponse>("/api/updates/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ containerId: updateContainerId, targetImage: updateTargetImage, validateUrl: validateURL })
      });
      connectUpdateEvents(response.jobId);
      await pollUpdate(response.jobId);
    } catch (e) { updateError = e instanceof Error ? e.message : "Update start failed"; }
  }

  onMount(() => {
    load();
  });

  onDestroy(() => {
    eventSource?.close();
    updateEventSource?.close();
    if (pollTimer) window.clearInterval(pollTimer);
    if (updatePollTimer) window.clearInterval(updatePollTimer);
  });
</script>

<div class="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 transition-colors duration-300">
  <Sidebar {currentRoute} onNavigate={(route) => currentRoute = route} />

  <main class="transition-all duration-300 {layoutStore.sidebarCollapsed ? 'pl-20' : 'pl-64'} min-h-screen">
    <div class="max-w-7xl mx-auto p-8">
      {#if currentRoute === 'dashboard'}
        <Dashboard {health} {summary} {malwareSummaries} {releaseSummary} {containers} {images} {events} onRefresh={load} />
      {:else if currentRoute === 'containers'}
        <Containers {containers} />
      {:else if currentRoute === 'images'}
        <Images {images} />
      {:else if currentRoute === 'security'}
        <Security 
          {summary} {malwareSummaries} {activeJob} {scanError} 
          bind:target bind:malwareTarget 
          onStartScan={startScan} onStartMalwareScan={startMalwareScan} 
        />
      {:else if currentRoute === 'intelligence'}
        <Intelligence {releaseSummary} {releaseError} bind:repo onAnalyze={loadReleaseSummary} />
      {:else if currentRoute === 'updates'}
        <Updates 
          {updateJob} {updateLive} {updateError} 
          bind:updateContainerId bind:updateTargetImage bind:validateURL 
          onStartUpdate={startUpdate} 
        />
      {:else if currentRoute === 'audit'}
        <AuditLog />
      {:else if currentRoute === 'settings'}
        <Settings />
      {/if}
    </div>
  </main>
</div>

<style>
  :global(body) {
    @apply antialiased overflow-x-hidden;
  }
</style>
