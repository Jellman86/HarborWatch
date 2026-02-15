<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import type {
    ContainerSummary,
    DockerEvent,
    HealthResponse,
    ImageSummary,
    ReleaseRiskSummary,
    ScanJobStatus,
    ScanStartResponse,
    ScanSummary,
    UpdateJobStatus,
    UpdateStartResponse,
    UpdateStepEvent
  } from "./lib/api-types";

  let health: HealthResponse | null = null;
  let containers: ContainerSummary[] = [];
  let images: ImageSummary[] = [];
  let events: DockerEvent[] = [];
  let summary: ScanSummary | null = null;
  let releaseSummary: ReleaseRiskSummary | null = null;
  let activeJob: ScanJobStatus | null = null;
  let updateJob: UpdateJobStatus | null = null;
  let updateLive: UpdateStepEvent[] = [];

  let target = "nginx:latest";
  let repo = "Jellman86/HarborWatch";
  let updateTargetImage = "ghcr.io/jellman86/harborwatch:latest";
  let validateURL = "http://localhost:18080/health";

  let error = "";
  let scanError = "";
  let releaseError = "";
  let updateError = "";

  let eventSource: EventSource | null = null;
  let updateEventSource: EventSource | null = null;
  let pollTimer: number | null = null;
  let updatePollTimer: number | null = null;

  const formatId = (id: string) => (id.length > 12 ? id.slice(0, 12) : id);
  const formatSize = (size: number) => size > 1024 * 1024 * 1024 ? `${(size / 1073741824).toFixed(2)} GB` : size > 1024 * 1024 ? `${(size / 1048576).toFixed(2)} MB` : `${size} B`;
  const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";

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
    try { summary = await fetchJSON<ScanSummary>("/api/scans/summary"); scanError = ""; }
    catch (e) { if (!(e instanceof Error && e.message.includes("(404)"))) scanError = e instanceof Error ? e.message : "Unknown error"; }
  }

  async function loadReleaseSummary() {
    try { releaseSummary = await fetchJSON<ReleaseRiskSummary>(`/api/releases/summary?repo=${encodeURIComponent(repo)}`); releaseError = ""; }
    catch (e) { releaseError = e instanceof Error ? e.message : "Release summary failed"; }
  }

  async function load() {
    error = "";
    try {
      const [h, c, i] = await Promise.all([
        fetchJSON<HealthResponse>("/health"),
        fetchJSON<ContainerSummary[]>("/api/docker/containers"),
        fetchJSON<ImageSummary[]>("/api/docker/images")
      ]);
      health = h; containers = c; images = i; connectEvents(); await loadSummary(); await loadReleaseSummary();
    } catch (e) { error = e instanceof Error ? e.message : "Unknown error"; }
  }

  async function pollJob(jobId: string) {
    if (pollTimer) window.clearInterval(pollTimer);
    pollTimer = window.setInterval(async () => {
      try {
        const job = await fetchJSON<ScanJobStatus>(`/api/scans/jobs/${jobId}`);
        activeJob = job;
        if (job.status === "completed" || job.status === "failed") { if (pollTimer) window.clearInterval(pollTimer); pollTimer = null; await loadSummary(); }
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
        body: JSON.stringify({ targetImage: updateTargetImage, validateUrl: validateURL })
      });
      connectUpdateEvents(response.jobId);
      await pollUpdate(response.jobId);
    } catch (e) { updateError = e instanceof Error ? e.message : "Update start failed"; }
  }

  onMount(() => { void load(); });
  onDestroy(() => {
    eventSource?.close();
    updateEventSource?.close();
    if (pollTimer) window.clearInterval(pollTimer);
    if (updatePollTimer) window.clearInterval(updatePollTimer);
  });
</script>

<main>
  <header><h1>HarborWatch</h1><button type="button" on:click={load}>Refresh</button></header>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if health}<section><h2>Health</h2><p>{health.service} is <strong>{health.status}</strong> ({health.version})</p></section>{/if}

  <section>
    <h2>Safe Update Pipeline</h2>
    <div class="scan-controls">
      <input bind:value={updateTargetImage} placeholder="target image" />
      <input bind:value={validateURL} placeholder="validate URL" />
      <button type="button" on:click={startUpdate}>Run Update</button>
    </div>
    {#if updateError}<p class="error" role="alert">{updateError}</p>{/if}
    {#if updateJob}
      <p>Update job <code>{updateJob.jobId}</code>: <strong>{updateJob.status}</strong>{#if updateJob.error} ({updateJob.error}){/if}</p>
      <ul>
        {#each updateJob.steps as s}<li>{s.step} - {s.status} - {s.message}</li>{:else}<li>No persisted steps yet</li>{/each}
      </ul>
    {/if}
    <h3>Live Progress</h3>
    <ul>
      {#each updateLive as e}<li>{e.step} - {e.status} - {e.message}</li>{:else}<li>Waiting for live events...</li>{/each}
    </ul>
  </section>

  <section>
    <h2>Release Intelligence</h2>
    <div class="scan-controls"><input bind:value={repo} placeholder="owner/repo" /><button type="button" on:click={loadReleaseSummary}>Analyze Releases</button></div>
    {#if releaseError}<p class="error" role="alert">{releaseError}</p>{/if}
    {#if releaseSummary}
      <ul>
        <li>Repo: <code>{releaseSummary.repo}</code></li><li>Latest tag: <code>{releaseSummary.latestTag || "n/a"}</code></li>
        <li>Releases analyzed: {releaseSummary.releasesAnalyzed}</li><li>Risk score: <strong>{releaseSummary.totalRisk}/100 ({riskBand(releaseSummary.totalRisk)})</strong></li>
        <li>Breaking change likely: <strong>{releaseSummary.breakingChangeLikely ? "Yes" : "No"}</strong></li>
      </ul>
      <h3>Highlighted Excerpts</h3>
      <ul>{#each releaseSummary.highlightedExcerpts as ex}<li><code>{ex.tag}</code> [{ex.weight}] - {ex.text}</li>{:else}<li>No excerpts</li>{/each}</ul>
    {/if}
  </section>

  <section>
    <h2>Vulnerability Summary</h2>
    <div class="scan-controls"><input bind:value={target} placeholder="image:tag" /><button type="button" on:click={startScan}>Run Trivy Scan</button></div>
    {#if scanError}<p class="error" role="alert">{scanError}</p>{/if}
    {#if activeJob}<p>Job <code>{activeJob.jobId}</code> for <code>{activeJob.target}</code>: <strong>{activeJob.status}</strong>{#if activeJob.error} ({activeJob.error}){/if}</p>{/if}
    {#if summary}
      <ul>
        <li>Target: <code>{summary.target}</code></li><li>Scanner: {summary.source}</li><li>Scanned: {new Date(summary.scannedAt * 1000).toLocaleString()}</li>
        <li>Critical: {summary.critical}</li><li>High: {summary.high}</li><li>Medium: {summary.medium}</li><li>Low: {summary.low}</li><li>Unknown: {summary.unknown}</li>
        <li>Total: {summary.total}</li><li>Risk Score: <strong>{summary.riskScore}/100 ({riskBand(summary.riskScore)})</strong></li>
      </ul>
    {:else}<p>No scan summary yet.</p>{/if}
  </section>

  <section><h2>Containers ({containers.length})</h2><ul>{#each containers as c}<li><strong>{formatId(c.id)}</strong> · {c.image} · {c.state} · {c.status}</li>{:else}<li>No containers found</li>{/each}</ul></section>
  <section><h2>Images ({images.length})</h2><ul>{#each images as i}<li><strong>{formatId(i.id)}</strong> · {i.repoTags?.[0] ?? "<none>:<none>"} · {formatSize(i.size)}</li>{:else}<li>No images found</li>{/each}</ul></section>
  <section><h2>Live Docker Events</h2><ul>{#each events as e}<li>{new Date(e.time * 1000).toLocaleTimeString()} · {e.type} · {e.action} · {formatId(e.id)}</li>{:else}<li>Waiting for events...</li>{/each}</ul></section>
</main>

<style>
  :global(body){margin:0;font-family:"IBM Plex Sans","Segoe UI",sans-serif;background:radial-gradient(circle at 20% 20%,#f8fbff,#e8f1ff 65%,#d8e8ff);color:#102a43}
  main{max-width:980px;margin:2rem auto;padding:1.25rem;border-radius:16px;background:rgba(255,255,255,.9);box-shadow:0 14px 40px rgba(16,42,67,.14)}
  header{display:flex;align-items:center;justify-content:space-between;gap:1rem} section{margin-top:1.5rem;padding-top:1rem;border-top:1px solid #d9e2ec}
  .scan-controls{display:flex;gap:.5rem;flex-wrap:wrap;margin-bottom:.6rem} input{min-width:18rem;border:1px solid #bcccdc;border-radius:8px;padding:.55rem .7rem}
  ul{padding-left:1.25rem;margin:0;display:grid;gap:.4rem} button{border:0;border-radius:10px;background:#0b7285;color:#fff;padding:.6rem .9rem;cursor:pointer;font-weight:600}
  button:hover{background:#086a7a}.error{background:#fff1f1;border:1px solid #ffd2d2;color:#9b1c1c;padding:.6rem .8rem;border-radius:8px}
</style>
