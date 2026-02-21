<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import ToastContainer from "./lib/components/ToastContainer.svelte";
  import GlobalProgress from "./lib/components/GlobalProgress.svelte";
  import { layoutStore } from "./lib/stores/layout.svelte";
  import { themeStore } from "./lib/stores/theme.svelte";
  import { configStore } from "./lib/stores/config.svelte";
  
  import type {
    ContainerSummary,
    DockerEvent,
    HealthResponse,
    ImageSummary,
    Settings
  } from "./lib/api-types";

  // Navigation State
  let currentRoute = $state("dashboard");
  let routeParams = $state<any>(null);

  function navigate(route: string, params: any = null) {
    if (route === "audit") {
      currentRoute = "diagnostics";
      routeParams = { preset: "audit" };
      return;
    }
    currentRoute = route;
    routeParams = params;
  }

  // Global Shared State
  let health = $state<HealthResponse | null>(null);
  let containers = $state<ContainerSummary[]>([]);
  let images = $state<ImageSummary[]>([]);
  let events = $state<DockerEvent[]>([]);
  let activeJobs = $state<any[]>([]);
  let error = $state("");

  let eventSource: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let dataPollTimer: ReturnType<typeof setInterval> | null = null;
  let reconnectDelayMs = 2000;
  const maxReconnectDelayMs = 30000;

  let idlePollCycles = 0;
  let currentPollInterval = 5000;

  async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
    const response = await fetch(url, init);
    if (!response.ok) throw new Error(`${url} failed (${response.status})`);
    return (await response.json()) as T;
  }

  async function loadActiveJobs() {
    try {
      // Use diagnostics snapshot to get active jobs efficiently
      const res = await fetch("/api/diagnostics/snapshot?includeFleet=false&logLimit=0&auditLimit=0");
      if (!res.ok) return;
      const data = await res.json();
      activeJobs = data.activeJobs || [];

      // Smart Polling Logic: Backoff if idle
      if (activeJobs.length === 0) {
        idlePollCycles++;
        if (idlePollCycles >= 3 && currentPollInterval === 5000) {
          updatePollInterval(30000);
        }
      } else {
        idlePollCycles = 0;
        if (currentPollInterval !== 5000) {
          updatePollInterval(5000);
        }
      }
    } catch (e) {
      // Silent error for background poll
    }
  }

  function updatePollInterval(ms: number) {
    if (pollTimer) clearInterval(pollTimer);
    currentPollInterval = ms;
    pollTimer = setInterval(loadActiveJobs, ms);
  }

  function connectEvents() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    eventSource?.close();
    eventSource = new EventSource("/api/docker/events");
    eventSource.onopen = () => {
      reconnectDelayMs = 2000;
    };
    eventSource.addEventListener("docker", (evt) => {
      try {
        const parsed = JSON.parse((evt as MessageEvent).data) as DockerEvent;
        events = [parsed, ...events].slice(0, 50);
      } catch (e) {
        console.error("Failed to parse docker event", e);
      }
    });

    eventSource.onerror = () => {
      eventSource?.close();
      if (reconnectTimer) return;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connectEvents();
      }, reconnectDelayMs);
      reconnectDelayMs = Math.min(reconnectDelayMs * 2, maxReconnectDelayMs);
    };
  }

  async function loadGlobalData() {
    error = "";
    try {
      const [h, c, i, s] = await Promise.all([
        fetchJSON<HealthResponse>("/health"),
        fetchJSON<ContainerSummary[]>("/api/docker/containers"),
        fetchJSON<ImageSummary[]>("/api/docker/images"),
        fetchJSON<Settings>("/api/settings")
      ]);
      health = h; 
      containers = c || []; 
      images = i || [];
      configStore.setSettings(s);
      connectEvents();
    } catch (e) { 
      error = e instanceof Error ? e.message : "Connection to backend failed"; 
    }
  }

  onMount(() => {
    loadGlobalData();
    loadActiveJobs();
    pollTimer = setInterval(loadActiveJobs, 5000);
    dataPollTimer = setInterval(loadGlobalData, 30000);
  });

  onDestroy(() => {
    if (reconnectTimer) clearTimeout(reconnectTimer);
    if (pollTimer) clearInterval(pollTimer);
    if (dataPollTimer) clearInterval(dataPollTimer);
    eventSource?.close();
  });
</script>

<div class="app-shell min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 transition-colors duration-300 bg-grain">
  <!-- Mobile Header -->
  <header class="md:hidden flex items-center justify-between p-4 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 sticky top-0 z-40 overflow-hidden">
    <div class="flex items-center gap-3">
      <button 
        onclick={() => layoutStore.openMobileSidebar()}
        class="p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
        aria-label="Open Navigation"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
      <div class="flex items-center gap-2">
        <img 
          src="/logo-64.png" 
          alt="HarborWatch" 
          class="h-8 w-8" 
        />
        <h1 class="text-sm font-black uppercase tracking-wider truncate max-w-[150px]">HarborWatch</h1>
      </div>
    </div>
    <div class="flex items-center gap-2">
      <!-- Placeholder for any right-side actions if needed in future -->
    </div>
  </header>

  <!-- Mobile Sidebar Backdrop -->
  {#if layoutStore.mobileSidebarOpen}
    <button 
      onclick={() => layoutStore.closeMobileSidebar()}
      class="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-40 md:hidden transition-opacity"
      aria-label="Close Navigation"
    ></button>
  {/if}

  <Sidebar {currentRoute} onNavigate={navigate} hasActiveJobs={activeJobs.length > 0} />

  <main
    class="app-main min-h-screen transition-[padding-left] duration-300"
    style={`--sidebar-offset:${layoutStore.sidebarCollapsed ? '5rem' : '16rem'}`}
  >
    <div class="sticky top-0 z-30">
      <GlobalProgress jobs={activeJobs} />
    </div>

    <div class="content-shell mx-auto w-full px-4 py-4 md:px-8 md:py-8">
      {#if error}
        <div class="mb-6 p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm font-bold flex items-center gap-3 animate-pulse">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
          </svg>
          {error}
        </div>
      {/if}

      {#if currentRoute === 'dashboard'}
        {#await import("./lib/pages/Dashboard.svelte") then Mod}
          <Mod.default {containers} {images} {events} onRefresh={loadGlobalData} onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'containers'}
        {#await import("./lib/pages/Containers.svelte") then Mod}
          <Mod.default {containers} params={routeParams} onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'container-detail'}
        {#await import("./lib/pages/ContainerPage.svelte") then Mod}
          <Mod.default id={routeParams.id} params={routeParams} onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'stacks'}
        {#await import("./lib/pages/Stacks.svelte") then Mod}
          <Mod.default onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'images'}
        {#await import("./lib/pages/Images.svelte") then Mod}
          <Mod.default {images} />
        {/await}
      {:else if currentRoute === 'automation'}
        {#await import("./lib/pages/Automation.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'doctor'}
        {#await import("./lib/pages/Doctor.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'intelligence'}
        {#await import("./lib/pages/Intelligence.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'updates'}
        {#await import("./lib/pages/Updates.svelte") then Mod}
          <Mod.default params={routeParams} />
        {/await}
      {:else if currentRoute === 'diagnostics'}
        {#await import("./lib/pages/Diagnostics.svelte") then Mod}
          <Mod.default params={routeParams} />
        {/await}
      {:else if currentRoute === 'ai-history'}
        {#await import("./lib/pages/AIHistory.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'settings'}
        {#await import("./lib/pages/Settings.svelte") then Mod}
          <Mod.default onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'about'}
        {#await import("./lib/pages/About.svelte") then Mod}
          <Mod.default />
        {/await}
      {/if}
    </div>
  </main>

  <ToastContainer />
</div>

<style>
  :global(body) {
    @apply antialiased overflow-x-hidden;
  }

  .content-shell {
    max-width: 120rem;
    @apply overflow-x-hidden;
  }

  .app-main {
    padding-left: 0;
    width: 100%;
    @apply overflow-x-hidden;
  }

  @media (min-width: 768px) {
    .app-main {
      padding-left: var(--sidebar-offset, 16rem);
      width: auto;
    }
  }
</style>
