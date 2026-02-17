<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import ToastContainer from "./lib/components/ToastContainer.svelte";
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

    eventSource.onerror = () => {
      console.warn("Docker events connection interrupted. Attempting to reconnect...");
      eventSource?.close();
      setTimeout(connectEvents, 5000);
    };
  }

  async function loadGlobalData() {
    error = "";
    try {
      const [h, c, i] = await Promise.all([
        fetchJSON<HealthResponse>("/health"),
        fetchJSON<ContainerSummary[]>("/api/docker/containers"),
        fetchJSON<ImageSummary[]>("/api/docker/images")
      ]);
      health = h; 
      containers = c || []; 
      images = i || [];
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

<div class="app-shell min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 transition-colors duration-300 bg-grain">
  <!-- Mobile Header -->
  <header class="md:hidden flex items-center justify-between p-4 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 sticky top-0 z-40">
    <div class="flex items-center gap-3">
      <div class="w-8 h-8 rounded-lg bg-brand-600 flex items-center justify-center text-white shadow-lg shadow-brand-500/20">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
        </svg>
      </div>
      <h1 class="text-sm font-bold uppercase tracking-wider">HarborWatch</h1>
    </div>
    <button 
      onclick={() => layoutStore.openMobileSidebar()}
      class="p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
      aria-label="Open Navigation"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </button>
  </header>

  <!-- Mobile Sidebar Backdrop -->
  {#if layoutStore.mobileSidebarOpen}
    <button 
      onclick={() => layoutStore.closeMobileSidebar()}
      class="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-40 md:hidden transition-opacity"
      aria-label="Close Navigation"
    ></button>
  {/if}

  <Sidebar {currentRoute} onNavigate={navigate} />

  <main
    class="app-main min-h-screen transition-[padding-left] duration-300"
    style={`--sidebar-offset:${layoutStore.sidebarCollapsed ? '5rem' : '16rem'}`}
  >
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
          <Mod.default {containers} {images} {events} onRefresh={loadGlobalData} />
        {/await}
      {:else if currentRoute === 'containers'}
        {#await import("./lib/pages/Containers.svelte") then Mod}
          <Mod.default {containers} onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'container-detail'}
        {#await import("./lib/pages/ContainerPage.svelte") then Mod}
          <Mod.default id={routeParams.id} onNavigate={navigate} />
        {/await}
      {:else if currentRoute === 'stacks'}
        {#await import("./lib/pages/Stacks.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'images'}
        {#await import("./lib/pages/Images.svelte") then Mod}
          <Mod.default {images} />
        {/await}
      {:else if currentRoute === 'security'}
        {#await import("./lib/pages/Security.svelte") then Mod}
          <Mod.default params={routeParams} />
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
      {:else if currentRoute === 'audit'}
        {#await import("./lib/pages/Audit.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'diagnostics'}
        {#await import("./lib/pages/Diagnostics.svelte") then Mod}
          <Mod.default />
        {/await}
      {:else if currentRoute === 'settings'}
        {#await import("./lib/pages/Settings.svelte") then Mod}
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
  }

  .app-main {
    padding-left: 0;
  }

  @media (min-width: 768px) {
    .app-main {
      padding-left: var(--sidebar-offset, 16rem);
    }
  }
</style>
