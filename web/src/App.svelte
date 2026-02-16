<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import Dashboard from "./lib/pages/Dashboard.svelte";
  import Containers from "./lib/pages/Containers.svelte";
  import Images from "./lib/pages/Images.svelte";
  import Security from "./lib/pages/Security.svelte";
  import Intelligence from "./lib/pages/Intelligence.svelte";
  import Updates from "./lib/pages/Updates.svelte";
  import Automation from "./lib/pages/Automation.svelte";
  import Doctor from "./lib/pages/Doctor.svelte";
  import AuditLog from "./lib/pages/AuditLog.svelte";
  import Settings from "./lib/pages/Settings.svelte";
  import { layoutStore } from "./lib/stores/layout.svelte";
  import { themeStore } from "./lib/stores/theme.svelte";
  
  import type {
    ContainerSummary,
    DockerEvent,
    HealthResponse,
    ImageSummary
  } from "./lib/api-types";

  // Navigation State
  let currentRoute = $state("dashboard");
  let routeParams = $state<any>(null);

  function navigate(route: string, params: any = null) {
    currentRoute = route;
    routeParams = params;
  }

  // Global Shared State
  let health = $state<HealthResponse | null>(null);
  let containers = $state<ContainerSummary[]>([]);
  let images = $state<ImageSummary[]>([]);
  let events = $state<DockerEvent[]>([]);
  let error = $state("");

  let eventSource: EventSource | null = null;

  async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
    const response = await fetch(url, init);
    if (!response.ok) throw new Error(`${url} failed (${response.status})`);
    return (await response.json()) as T;
  }

  function connectEvents() {
    eventSource?.close();
    eventSource = new EventSource("/api/docker/events");
    eventSource.addEventListener("docker", (evt) => {
      try {
        const parsed = JSON.parse((evt as MessageEvent).data) as DockerEvent;
        events = [parsed, ...events].slice(0, 50);
      } catch (e) {
        console.error("Failed to parse docker event", e);
      }
    });
  }

  async function loadGlobalData() {
    error = "";
    try {
      const [h, c, i] = await Promise.all([
        fetchJSON<HealthResponse>("/health"),
        fetchJSON<ContainerSummary[]>("/api/docker/containers"),
        fetchJSON<ImageSummary[]>("/api/docker/images")
      ]);
      health = h; containers = c; images = i;
      connectEvents();
    } catch (e) { 
      error = e instanceof Error ? e.message : "Connection to backend failed"; 
    }
  }

  onMount(() => {
    loadGlobalData();
  });

  onDestroy(() => {
    eventSource?.close();
  });
</script>

<div class="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 transition-colors duration-300">
  <Sidebar {currentRoute} onNavigate={navigate} />

  <main class="transition-all duration-300 {layoutStore.sidebarCollapsed ? 'pl-20' : 'pl-64'} min-h-screen">
    <div class="max-w-7xl mx-auto p-8">
      {#if error}
        <div class="mb-6 p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm font-bold flex items-center gap-3 animate-pulse">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
          </svg>
          {error}
        </div>
      {/if}

      {#if currentRoute === 'dashboard'}
        <Dashboard {health} {containers} {images} {events} onRefresh={loadGlobalData} />
      {:else if currentRoute === 'containers'}
        <Containers {containers} onNavigate={navigate} />
      {:else if currentRoute === 'images'}
        <Images {images} />
      {:else if currentRoute === 'security'}
        <Security params={routeParams} />
      {:else if currentRoute === 'automation'}
        <Automation />
      {:else if currentRoute === 'doctor'}
        <Doctor />
      {:else if currentRoute === 'intelligence'}
        <Intelligence />
      {:else if currentRoute === 'updates'}
        <Updates params={routeParams} />
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
