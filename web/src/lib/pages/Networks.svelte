<script lang="ts">
    import { onMount } from "svelte";
    import { toasts } from "../stores/ToastStore";

    let { onNavigate } = $props<{
        onNavigate: (route: string, params?: any) => void;
    }>();

    type NetworkNode = {
        id: string;
        name: string;
        driver?: string;
        scope?: string;
        internal?: boolean;
        attachable?: boolean;
        ingress?: boolean;
        ipam?: Array<{ subnet?: string; gateway?: string }>;
        labels?: Record<string, string>;
    };
    type ContainerNode = {
        id: string;
        name?: string;
        image?: string;
        state?: string;
        labels?: Record<string, string>;
    };
    type Edge = {
        networkId: string;
        containerId: string;
        endpointId?: string;
        endpointName?: string;
        macAddress?: string;
        ipv4Address?: string;
        ipv6Address?: string;
    };
    type TopologySnapshot = {
        generatedAt: number;
        networks: NetworkNode[];
        containers: ContainerNode[];
        edges: Edge[];
    };

    type PositionedNetwork = NetworkNode & { x: number; y: number; width: number; height: number };
    type PositionedContainer = ContainerNode & { x: number; y: number; width: number; height: number };
    type PositionedEdge = Edge & { key: string; path: string; midX: number; midY: number };
    type ConnectionRow = {
        key: string;
        edge: PositionedEdge;
        network: NetworkNode | null;
        container: ContainerNode | null;
        networkName: string;
        containerName: string;
        driver: string;
    };

    let loading = $state(true);
    let error = $state("");
    let refreshing = $state(false);
    let topology = $state<TopologySnapshot>({ generatedAt: 0, networks: [], containers: [], edges: [] });

    let selectedKind = $state<"network" | "container" | "edge" | null>(null);
    let selectedId = $state("");
    let selectedEdgeKey = $state("");
    let hoveredEdgeKey = $state("");
    let networkSearch = $state("");
    let containerSearch = $state("");
    let driverFilter = $state("all");

    async function loadTopology(options?: { quiet?: boolean }) {
        const quiet = !!options?.quiet;
        if (!quiet) loading = true;
        refreshing = quiet;
        error = "";
        try {
            const res = await fetch("/api/networks/topology");
            const data = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(data.message || "Failed to load network topology");
            topology = {
                generatedAt: Number(data.generatedAt || 0),
                networks: Array.isArray(data.networks) ? data.networks : [],
                containers: Array.isArray(data.containers) ? data.containers : [],
                edges: Array.isArray(data.edges) ? data.edges : []
            };
            if (!quiet) {
                toasts.success("Network topology refreshed");
            }
            if (selectedKind === "network" && !topology.networks.some((n) => n.id === selectedId)) {
                selectedKind = null; selectedId = "";
            }
            if (selectedKind === "container" && !topology.containers.some((c) => c.id === selectedId)) {
                selectedKind = null; selectedId = "";
            }
            if (selectedKind === "edge" && !topology.edges.some((e, i) => edgeKey(e, i) === selectedEdgeKey)) {
                selectedKind = null; selectedEdgeKey = "";
            }
        } catch (e) {
            error = e instanceof Error ? e.message : "Failed to load topology";
            if (!quiet) toasts.error(error);
        } finally {
            loading = false;
            refreshing = false;
        }
    }

    function edgeKey(e: Edge, index: number): string {
        return `${e.networkId}|${e.containerId}|${e.endpointId || index}`;
    }

    function containsCI(value: unknown, query: string): boolean {
        if (!query) return true;
        return String(value || "").toLowerCase().includes(query);
    }

    function containerLabel(c: ContainerNode | null | undefined): string {
        if (!c) return "Unknown Container";
        return c.name || c.id.slice(0, 12);
    }

    function networkLabel(n: NetworkNode | null | undefined): string {
        if (!n) return "Unknown Network";
        return n.name || n.id.slice(0, 12);
    }

    const hasActiveFilters = $derived.by(() =>
        !!networkSearch.trim() || !!containerSearch.trim() || driverFilter !== "all"
    );

    const availableDrivers = $derived.by(() => {
        const values = new Set<string>();
        for (const n of topology.networks) {
            const d = String(n.driver || "").trim().toLowerCase();
            if (d) values.add(d);
        }
        return Array.from(values).sort((a, b) => a.localeCompare(b));
    });

    const visibleTopology = $derived.by(() => {
        const raw = topology;
        const networkQ = networkSearch.trim().toLowerCase();
        const containerQ = containerSearch.trim().toLowerCase();
        const driverQ = driverFilter.trim().toLowerCase();
        const driverActive = driverQ !== "" && driverQ !== "all";
        if (!networkQ && !containerQ && !driverActive) return raw;

        const matchedNetworkIDs = new Set<string>();
        for (const n of raw.networks) {
            const driverMatches = !driverActive || String(n.driver || "").toLowerCase() === driverQ;
            if (!driverMatches) continue;
            const networkTextMatches =
                !networkQ ||
                containsCI(n.name, networkQ) ||
                containsCI(n.id, networkQ) ||
                containsCI(n.driver, networkQ) ||
                containsCI(n.scope, networkQ) ||
                (n.ipam || []).some((ip) => containsCI(ip.subnet, networkQ) || containsCI(ip.gateway, networkQ));
            if (networkTextMatches) matchedNetworkIDs.add(n.id);
        }

        const matchedContainerIDs = new Set<string>();
        for (const c of raw.containers) {
            const containerTextMatches =
                !containerQ ||
                containsCI(c.name, containerQ) ||
                containsCI(c.id, containerQ) ||
                containsCI(c.image, containerQ) ||
                containsCI(c.state, containerQ);
            if (containerTextMatches) matchedContainerIDs.add(c.id);
        }

        const edges = raw.edges.filter(
            (e) => matchedNetworkIDs.has(e.networkId) && matchedContainerIDs.has(e.containerId)
        );
        const visibleNetworkIDs = new Set<string>();
        const visibleContainerIDs = new Set<string>();
        for (const e of edges) {
            visibleNetworkIDs.add(e.networkId);
            visibleContainerIDs.add(e.containerId);
        }
        if (networkQ || driverActive) {
            for (const id of matchedNetworkIDs) visibleNetworkIDs.add(id);
        }
        if (containerQ) {
            for (const id of matchedContainerIDs) visibleContainerIDs.add(id);
        }

        return {
            generatedAt: raw.generatedAt,
            networks: raw.networks.filter((n) => visibleNetworkIDs.has(n.id)),
            containers: raw.containers.filter((c) => visibleContainerIDs.has(c.id)),
            edges
        };
    });

    const graphMetrics = $derived.by(() => {
        const networks = visibleTopology.networks.length;
        const containers = visibleTopology.containers.length;
        const rowGap = 88;
        const padY = 48;
        const count = Math.max(networks, containers, 1);
        const height = padY * 2 + (count - 1) * rowGap + 76;
        const width = 1180;
        return { width, height, rowGap, padY };
    });

    const positioned = $derived.by(() => {
        const metrics = graphMetrics;
        const networkWidth = 320;
        const containerWidth = 360;
        const nodeHeight = 68;
        const networkX = 64;
        const containerX = metrics.width - containerWidth - 64;
        const networks: PositionedNetwork[] = visibleTopology.networks.map((n, i) => ({
            ...n,
            x: networkX,
            y: metrics.padY + i * metrics.rowGap,
            width: networkWidth,
            height: nodeHeight
        }));
        const containers: PositionedContainer[] = visibleTopology.containers.map((c, i) => ({
            ...c,
            x: containerX,
            y: metrics.padY + i * metrics.rowGap,
            width: containerWidth,
            height: nodeHeight
        }));
        const networkById = new Map(networks.map((n) => [n.id, n]));
        const containerById = new Map(containers.map((c) => [c.id, c]));
        const edges: PositionedEdge[] = visibleTopology.edges.map((e, i) => {
            const n = networkById.get(e.networkId);
            const c = containerById.get(e.containerId);
            if (!n || !c) {
                return { ...e, key: edgeKey(e, i), path: "", midX: 0, midY: 0 };
            }
            const x1 = n.x + n.width;
            const y1 = n.y + n.height / 2;
            const x2 = c.x;
            const y2 = c.y + c.height / 2;
            const cx1 = x1 + 140;
            const cx2 = x2 - 140;
            const path = `M ${x1} ${y1} C ${cx1} ${y1}, ${cx2} ${y2}, ${x2} ${y2}`;
            return { ...e, key: edgeKey(e, i), path, midX: (x1 + x2) / 2, midY: (y1 + y2) / 2 };
        });
        return { networks, containers, edges };
    });

    const selectedNetwork = $derived(
        selectedKind === "network" ? visibleTopology.networks.find((n) => n.id === selectedId) || null : null
    );
    const selectedContainer = $derived(
        selectedKind === "container" ? visibleTopology.containers.find((c) => c.id === selectedId) || null : null
    );
    const selectedEdge = $derived(
        selectedKind === "edge" ? positioned.edges.find((e) => e.key === selectedEdgeKey) || null : null
    );

    const relatedEdgeKeys = $derived.by(() => {
        const keys = new Set<string>();
        if (selectedKind === "network") {
            for (const e of positioned.edges) if (e.networkId === selectedId) keys.add(e.key);
        } else if (selectedKind === "container") {
            for (const e of positioned.edges) if (e.containerId === selectedId) keys.add(e.key);
        } else if (selectedKind === "edge" && selectedEdgeKey) {
            keys.add(selectedEdgeKey);
        }
        return keys;
    });

    const relatedNetworkIDs = $derived.by(() => {
        const ids = new Set<string>();
        if (selectedKind === "network" && selectedId) ids.add(selectedId);
        for (const e of positioned.edges) if (relatedEdgeKeys.has(e.key)) ids.add(e.networkId);
        return ids;
    });

    const relatedContainerIDs = $derived.by(() => {
        const ids = new Set<string>();
        if (selectedKind === "container" && selectedId) ids.add(selectedId);
        for (const e of positioned.edges) if (relatedEdgeKeys.has(e.key)) ids.add(e.containerId);
        return ids;
    });

    const connectionRows = $derived.by(() => {
        const networksByID = new Map(visibleTopology.networks.map((n) => [n.id, n]));
        const containersByID = new Map(visibleTopology.containers.map((c) => [c.id, c]));
        return positioned.edges
            .map((edge): ConnectionRow => {
                const network = networksByID.get(edge.networkId) || null;
                const container = containersByID.get(edge.containerId) || null;
                return {
                    key: edge.key,
                    edge,
                    network,
                    container,
                    networkName: networkLabel(network),
                    containerName: containerLabel(container),
                    driver: String(network?.driver || "unknown")
                };
            })
            .sort((a, b) =>
                a.networkName.localeCompare(b.networkName) ||
                a.containerName.localeCompare(b.containerName) ||
                a.key.localeCompare(b.key)
            );
    });

    function selectNetwork(id: string) {
        selectedKind = "network";
        selectedId = id;
        selectedEdgeKey = "";
    }
    function selectContainer(id: string) {
        selectedKind = "container";
        selectedId = id;
        selectedEdgeKey = "";
    }
    function selectEdge(key: string) {
        selectedKind = "edge";
        selectedEdgeKey = key;
        selectedId = "";
    }
    function clearSelection() {
        selectedKind = null;
        selectedId = "";
        selectedEdgeKey = "";
    }

    function formatWhen(ts: number | undefined): string {
        const n = Number(ts || 0);
        if (!n) return "Unknown";
        return new Date(n * 1000).toLocaleString();
    }

    function edgeLabel(e: Edge): string {
        const parts: string[] = [];
        if (e.ipv4Address) parts.push(e.ipv4Address);
        if (e.ipv6Address) parts.push(e.ipv6Address);
        if (e.macAddress) parts.push(e.macAddress);
        return parts.join(" | ") || (e.endpointId || "Endpoint");
    }

    function compactEdgeDetail(e: Edge): string {
        if (e.ipv4Address || e.ipv6Address) return [e.ipv4Address, e.ipv6Address].filter(Boolean).join(" | ");
        if (e.macAddress) return e.macAddress;
        return e.endpointId || "endpoint";
    }

    function networkDriverClass(driver: string | undefined): string {
        const d = String(driver || "").toLowerCase();
        if (d === "overlay") return "overlay";
        if (d === "macvlan" || d === "ipvlan") return "l2";
        return "bridge";
    }

    function networkFill(driver: string | undefined): string {
        const kind = networkDriverClass(driver);
        if (kind === "overlay") return "rgba(6,182,212,0.10)";
        if (kind === "l2") return "rgba(245,158,11,0.10)";
        return "rgba(16,185,129,0.10)";
    }

    function networkStroke(driver: string | undefined, selected: boolean): string {
        if (selected) return "rgb(6 182 212)";
        const kind = networkDriverClass(driver);
        if (kind === "overlay") return "rgba(6,182,212,0.35)";
        if (kind === "l2") return "rgba(245,158,11,0.35)";
        return "rgba(16,185,129,0.35)";
    }

    $effect(() => {
        if (selectedKind === "network" && selectedId && !visibleTopology.networks.some((n) => n.id === selectedId)) {
            clearSelection();
        } else if (
            selectedKind === "container" &&
            selectedId &&
            !visibleTopology.containers.some((c) => c.id === selectedId)
        ) {
            clearSelection();
        } else if (
            selectedKind === "edge" &&
            selectedEdgeKey &&
            !positioned.edges.some((e) => e.key === selectedEdgeKey)
        ) {
            clearSelection();
        }
    });

    onMount(() => {
        loadTopology();
    });
</script>

<div class="space-y-6">
    <div class="relative overflow-hidden rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-5 md:p-6">
        <div class="absolute inset-0 opacity-[0.05] pointer-events-none" style="background-image: radial-gradient(circle at 1px 1px, currentColor 1px, transparent 0); background-size: 20px 20px;"></div>
        <div class="relative flex flex-col lg:flex-row lg:items-center justify-between gap-4">
            <div class="border-l-4 border-cyan-500 pl-4">
                <div class="flex flex-wrap items-center gap-2">
                    <h2 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white uppercase">Network Topology</h2>
                    <span class="px-2 py-1 rounded-full bg-slate-100 dark:bg-slate-800 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-700">Read-Only</span>
                </div>
                <p class="text-xs text-slate-500 mt-1">User-defined Docker networks only. Built from live Docker network topology (actual attachments and endpoint addressing).</p>
                <p class="text-[11px] text-slate-400 mt-1">Last refresh: <span class="font-bold text-slate-500">{formatWhen(topology.generatedAt)}</span></p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <button
                    onclick={clearSelection}
                    class="px-3 py-2 rounded-xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 hover:border-slate-300"
                >
                    Clear Focus
                </button>
                <button
                    onclick={() => loadTopology({ quiet: true })}
                    disabled={refreshing}
                    class="px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest shadow-lg shadow-cyan-500/20 flex items-center gap-2"
                >
                    {#if refreshing}
                        <div class="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                    {/if}
                    {refreshing ? "Refreshing..." : "Refresh Topology"}
                </button>
            </div>
        </div>
    </div>

    {#if loading}
        <div class="rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-16 text-center space-y-4">
            <div class="mx-auto w-10 h-10 border-4 border-cyan-500 border-t-transparent rounded-full animate-spin"></div>
            <p class="text-[11px] font-black uppercase tracking-widest text-slate-400">Scanning Docker Networks...</p>
        </div>
    {:else if error}
        <div class="rounded-3xl border border-rose-200 dark:border-rose-900/30 bg-rose-50 dark:bg-rose-900/10 p-8 space-y-3">
            <p class="text-sm font-black text-rose-700 dark:text-rose-300">Failed to Load Network Topology</p>
            <p class="text-sm text-rose-700/80 dark:text-rose-300/80">{error}</p>
            <button onclick={() => loadTopology()} class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-[10px] font-black uppercase tracking-widest">Retry</button>
        </div>
    {:else if topology.networks.length === 0}
        <div class="rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 p-14 text-center space-y-3">
            <p class="text-base font-black text-slate-800 dark:text-slate-100">No User-Defined Networks Found</p>
            <p class="text-sm text-slate-500 max-w-2xl mx-auto">This view currently hides default Docker networks (`bridge`, `host`, `none`). Create or connect containers to a custom Docker network to visualize topology here.</p>
            <button onclick={() => onNavigate('containers')} class="px-4 py-2 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 text-[10px] font-black uppercase tracking-widest">Open Fleet</button>
        </div>
    {:else}
        <div class="space-y-6">
            <section class="rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-4 md:p-5">
                <div class="flex flex-col gap-4">
                    <div class="flex flex-wrap items-center justify-between gap-3">
                        <div>
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Filters</p>
                            <p class="text-xs text-slate-500">Filter the graph and fallback list by network, container, and driver.</p>
                        </div>
                        {#if hasActiveFilters}
                            <button
                                onclick={() => {
                                    networkSearch = "";
                                    containerSearch = "";
                                    driverFilter = "all";
                                }}
                                class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 hover:border-cyan-400"
                            >
                                Clear Filters
                            </button>
                        {/if}
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                        <label class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50/80 dark:bg-slate-800/40 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Network Search</p>
                            <input
                                bind:value={networkSearch}
                                placeholder="name, subnet, scope..."
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm text-slate-800 dark:text-slate-100 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                            />
                        </label>
                        <label class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50/80 dark:bg-slate-800/40 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Container Search</p>
                            <input
                                bind:value={containerSearch}
                                placeholder="name, image, state..."
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm text-slate-800 dark:text-slate-100 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                            />
                        </label>
                        <label class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50/80 dark:bg-slate-800/40 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Driver</p>
                            <select
                                bind:value={driverFilter}
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm text-slate-800 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                            >
                                <option value="all">All Drivers</option>
                                {#each availableDrivers as driver}
                                    <option value={driver}>{driver}</option>
                                {/each}
                            </select>
                        </label>
                    </div>
                </div>
            </section>

            <div class="grid grid-cols-1 xl:grid-cols-[minmax(0,2fr)_380px] gap-6">
                <div class="space-y-6">
                    <section class="relative rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm overflow-hidden">
                <div class="absolute inset-0 pointer-events-none opacity-[0.06]" style="background-image: linear-gradient(to right, currentColor 1px, transparent 1px), linear-gradient(to bottom, currentColor 1px, transparent 1px); background-size: 40px 40px;"></div>
                <div class="relative p-4 border-b border-slate-100 dark:border-slate-800 flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Graph</p>
                        <p class="text-xs text-slate-500">Networks on the left, containers on the right. Hover or click links to reveal endpoint addressing.</p>
                    </div>
                    <div class="flex flex-wrap items-center gap-2 text-[10px]">
                        {#if hasActiveFilters}
                            <span class="px-2 py-1 rounded-full bg-cyan-50 text-cyan-700 dark:bg-cyan-900/20 dark:text-cyan-300 border border-cyan-200/60 dark:border-cyan-800/30 font-black uppercase tracking-widest">
                                Filtered View
                            </span>
                        {/if}
                        <span class="px-2 py-1 rounded-full bg-emerald-100/70 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300 border border-emerald-200/60 dark:border-emerald-800/30 font-black uppercase tracking-widest">Network</span>
                        <span class="px-2 py-1 rounded-full bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border border-slate-200 dark:border-slate-700 font-black uppercase tracking-widest">Container</span>
                    </div>
                </div>
                <div class="overflow-auto">
                    {#if hasActiveFilters && visibleTopology.networks.length === 0 && visibleTopology.containers.length === 0}
                        <div class="p-10 text-center space-y-2">
                            <p class="text-sm font-black text-slate-700 dark:text-slate-200">No Topology Nodes Match Current Filters</p>
                            <p class="text-xs text-slate-500">Adjust the network search, container search, or driver filter to widen the visible topology.</p>
                        </div>
                    {:else}
                        <svg
                            viewBox={`0 0 ${graphMetrics.width} ${graphMetrics.height}`}
                            class="min-w-[1000px] w-full h-auto"
                            aria-label="Docker network topology graph"
                        >
                        <defs>
                            <filter id="glowEdge" x="-30%" y="-30%" width="160%" height="160%">
                                <feGaussianBlur stdDeviation="2.5" result="blur" />
                                <feMerge>
                                    <feMergeNode in="blur" />
                                    <feMergeNode in="SourceGraphic" />
                                </feMerge>
                            </filter>
                        </defs>

                        <text x="74" y="28" class="fill-slate-400 text-[12px] font-black uppercase tracking-[0.2em]">Networks</text>
                        <text x={graphMetrics.width - 220} y="28" class="fill-slate-400 text-[12px] font-black uppercase tracking-[0.2em]">Containers</text>

                        {#each positioned.edges as e}
                            {@const isSelected = selectedKind === "edge" && selectedEdgeKey === e.key}
                            {@const isHovered = hoveredEdgeKey === e.key}
                            {@const related = relatedEdgeKeys.has(e.key)}
                            {@const dimmed = (selectedKind !== null) && !related}
                            {#if e.path}
                                <path
                                    d={e.path}
                                    fill="none"
                                    stroke={isSelected || isHovered ? "rgb(6 182 212)" : "rgb(148 163 184)"}
                                    stroke-opacity={dimmed ? 0.18 : (isSelected || isHovered ? 0.95 : 0.45)}
                                    stroke-width={isSelected || isHovered ? "3.25" : "2"}
                                    filter={isSelected || isHovered ? "url(#glowEdge)" : undefined}
                                    class="transition-all duration-200 cursor-pointer"
                                    role="button"
                                    tabindex="0"
                                    aria-label={`Connection ${e.containerId} to ${e.networkId}`}
                                    onmouseenter={() => hoveredEdgeKey = e.key}
                                    onmouseleave={() => hoveredEdgeKey = ""}
                                    onclick={() => selectEdge(e.key)}
                                    onkeydown={(evt) => (evt.key === "Enter" || evt.key === " ") && (evt.preventDefault(), selectEdge(e.key))}
                                />
                                {#if isSelected || isHovered}
                                    <g transform={`translate(${e.midX}, ${e.midY - 12})`}>
                                        <rect x="-170" y="-14" width="340" height="28" rx="10" class="fill-white/95 dark:fill-slate-900/95 stroke-cyan-300/60 dark:stroke-cyan-800/50" />
                                        <text text-anchor="middle" dominant-baseline="middle" class="fill-slate-700 dark:fill-slate-200 text-[10px] font-bold">
                                            {edgeLabel(e)}
                                        </text>
                                    </g>
                                {/if}
                            {/if}
                        {/each}

                        {#each positioned.networks as n}
                            {@const selected = selectedKind === "network" && selectedId === n.id}
                            {@const related = relatedNetworkIDs.has(n.id)}
                            {@const dimmed = selectedKind !== null && !related}
                            <g
                                transform={`translate(${n.x}, ${n.y})`}
                                class="cursor-pointer"
                                role="button"
                                tabindex="0"
                                aria-label={`Network ${n.name}`}
                                onclick={() => selectNetwork(n.id)}
                                onkeydown={(evt) => (evt.key === "Enter" || evt.key === " ") && (evt.preventDefault(), selectNetwork(n.id))}
                            >
                                <rect
                                    width={n.width}
                                    height={n.height}
                                    rx="18"
                                    stroke-width="2"
                                    fill={networkFill(n.driver)}
                                    stroke={networkStroke(n.driver, selected)}
                                    opacity={dimmed ? "0.28" : "1"}
                                />
                                <text x="16" y="26" class="fill-slate-900 dark:fill-white text-[15px] font-black tracking-tight">
                                    {n.name}
                                </text>
                                <text x="16" y="44" class="fill-slate-500 dark:fill-slate-400 text-[10px] font-bold uppercase tracking-widest">
                                    {n.driver || "driver?"} {n.scope ? ` | ${n.scope}` : ""}
                                </text>
                                {#if n.ipam && n.ipam[0]?.subnet}
                                    <text x="16" y="60" class="fill-slate-400 dark:fill-slate-500 text-[10px] font-mono">
                                        {n.ipam[0].subnet}
                                    </text>
                                {/if}
                            </g>
                        {/each}

                        {#each positioned.containers as c}
                            {@const selected = selectedKind === "container" && selectedId === c.id}
                            {@const related = relatedContainerIDs.has(c.id)}
                            {@const dimmed = selectedKind !== null && !related}
                            <g
                                transform={`translate(${c.x}, ${c.y})`}
                                class="cursor-pointer"
                                role="button"
                                tabindex="0"
                                aria-label={`Container ${c.name || c.id}`}
                                onclick={() => selectContainer(c.id)}
                                onkeydown={(evt) => (evt.key === "Enter" || evt.key === " ") && (evt.preventDefault(), selectContainer(c.id))}
                            >
                                <rect width={c.width} height={c.height} rx="18" class={`fill-white dark:fill-slate-950 stroke-2 transition-all duration-200 ${selected ? 'stroke-cyan-500' : 'stroke-slate-200 dark:stroke-slate-800'}`} opacity={dimmed ? "0.28" : "1"} />
                                <rect x="1" y="1" width={c.width - 2} height={c.height - 2} rx="17" class="fill-slate-50 dark:fill-slate-900/70 stroke-transparent" opacity={dimmed ? "0.28" : "1"} />
                                <text x="16" y="26" class="fill-slate-900 dark:fill-white text-[14px] font-black tracking-tight">
                                    {c.name || c.id.slice(0, 12)}
                                </text>
                                <text x="16" y="44" class="fill-slate-500 dark:fill-slate-400 text-[10px] font-bold uppercase tracking-widest">
                                    {c.state || "unknown"} {c.image ? " | image" : ""}
                                </text>
                                {#if c.image}
                                    <text x="16" y="60" class="fill-slate-400 dark:fill-slate-500 text-[10px] font-mono">
                                        {c.image.length > 46 ? c.image.slice(0, 43) + "..." : c.image}
                                    </text>
                                {/if}
                            </g>
                        {/each}
                        </svg>
                    {/if}
                </div>
            </section>

                    <section class="rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm overflow-hidden">
                        <div class="p-4 border-b border-slate-100 dark:border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Connection List</p>
                                <p class="text-xs text-slate-500">Fallback list view for dense topologies. Uses the same current filter scope as the graph.</p>
                            </div>
                            <span class="px-2 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                                {connectionRows.length} Visible Links
                            </span>
                        </div>
                        {#if connectionRows.length === 0}
                            <div class="p-8 text-center space-y-2">
                                <p class="text-sm font-black text-slate-700 dark:text-slate-200">No Connections Match Current Filters</p>
                                <p class="text-xs text-slate-500">Try broadening network/container search or changing the driver filter.</p>
                            </div>
                        {:else}
                            <div class="overflow-auto">
                                <div class="min-w-[860px]">
                                    <div class="grid grid-cols-[minmax(0,1.2fr)_120px_minmax(0,1.25fr)_110px_minmax(0,1.5fr)] gap-3 px-4 py-3 border-b border-slate-100 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-800/30 text-[9px] font-black uppercase tracking-widest text-slate-400">
                                        <div>Network</div>
                                        <div>Driver</div>
                                        <div>Container</div>
                                        <div>State</div>
                                        <div>Endpoint</div>
                                    </div>
                                    <div class="divide-y divide-slate-100 dark:divide-slate-800">
                                        {#each connectionRows as row}
                                            {@const edgeSelected = selectedKind === "edge" && selectedEdgeKey === row.key}
                                            <div
                                                onclick={() => selectEdge(row.key)}
                                                onkeydown={(evt) => (evt.key === "Enter" || evt.key === " ") && (evt.preventDefault(), selectEdge(row.key))}
                                                role="button"
                                                tabindex="0"
                                                class={`w-full text-left grid grid-cols-[minmax(0,1.2fr)_120px_minmax(0,1.25fr)_110px_minmax(0,1.5fr)] gap-3 px-4 py-3 transition-colors cursor-pointer ${edgeSelected ? 'bg-cyan-50/70 dark:bg-cyan-900/10' : 'hover:bg-slate-50/70 dark:hover:bg-slate-800/30'}`}
                                            >
                                                <div class="min-w-0">
                                                    <p class="text-[11px] font-black text-slate-900 dark:text-white truncate">{row.networkName}</p>
                                                    <p class="text-[10px] text-slate-500 font-mono truncate">{row.network?.id || row.edge.networkId}</p>
                                                </div>
                                                <div class="min-w-0">
                                                    <span class="inline-flex px-2 py-1 rounded-lg border text-[9px] font-black uppercase tracking-widest {networkDriverClass(row.driver) === 'overlay' ? 'border-cyan-200 text-cyan-700 dark:border-cyan-900/40 dark:text-cyan-300 bg-cyan-50/60 dark:bg-cyan-900/10' : networkDriverClass(row.driver) === 'l2' ? 'border-amber-200 text-amber-700 dark:border-amber-900/40 dark:text-amber-300 bg-amber-50/60 dark:bg-amber-900/10' : 'border-emerald-200 text-emerald-700 dark:border-emerald-900/40 dark:text-emerald-300 bg-emerald-50/60 dark:bg-emerald-900/10'}">
                                                        {row.driver}
                                                    </span>
                                                </div>
                                                <div class="min-w-0">
                                                    <p class="text-[11px] font-black text-slate-900 dark:text-white truncate">{row.containerName}</p>
                                                    <p class="text-[10px] text-slate-500 font-mono truncate">{row.container?.image || row.container?.id || row.edge.containerId}</p>
                                                </div>
                                                <div class="min-w-0">
                                                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">{row.container?.state || "unknown"}</p>
                                                </div>
                                                <div class="min-w-0">
                                                    <p class="text-[10px] font-mono text-slate-600 dark:text-slate-300 truncate">{compactEdgeDetail(row.edge)}</p>
                                                    <div class="mt-1 flex items-center gap-2">
                                                        <button
                                                            type="button"
                                                            onclick={(evt) => {
                                                                evt.stopPropagation();
                                                                selectNetwork(row.edge.networkId);
                                                            }}
                                                            class="px-2 py-1 rounded-lg border border-slate-200 dark:border-slate-700 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 hover:border-cyan-400"
                                                        >
                                                            Network
                                                        </button>
                                                        <button
                                                            type="button"
                                                            onclick={(evt) => {
                                                                evt.stopPropagation();
                                                                selectContainer(row.edge.containerId);
                                                            }}
                                                            class="px-2 py-1 rounded-lg border border-slate-200 dark:border-slate-700 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 hover:border-cyan-400"
                                                        >
                                                            Container
                                                        </button>
                                                    </div>
                                                </div>
                                            </div>
                                        {/each}
                                    </div>
                                </div>
                            </div>
                        {/if}
                    </section>
                </div>

                <aside class="rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-5 space-y-4">
                <div class="flex items-center justify-between gap-3">
                    <div>
                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Inspector</p>
                        <p class="text-xs text-slate-500">Select a network, container, or connection for runtime details.</p>
                    </div>
                    <span class="px-2 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                        {visibleTopology.networks.length}N / {visibleTopology.containers.length}C / {visibleTopology.edges.length}E
                        {#if hasActiveFilters}
                            <span class="text-slate-400"> (filtered)</span>
                        {/if}
                    </span>
                </div>

                {#if selectedNetwork}
                    <div class="space-y-4">
                        <div class="rounded-2xl border border-emerald-200/70 dark:border-emerald-900/40 bg-emerald-50/70 dark:bg-emerald-900/10 p-4">
                            <p class="text-[10px] font-black uppercase tracking-widest text-emerald-700 dark:text-emerald-300">Network</p>
                            <p class="mt-1 text-lg font-black text-slate-900 dark:text-white break-all">{selectedNetwork.name}</p>
                            <p class="mt-1 text-[10px] font-mono text-slate-500 break-all">{selectedNetwork.id}</p>
                        </div>
                        <div class="grid grid-cols-2 gap-3 text-[11px]">
                            <div class="rounded-xl bg-slate-50 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 p-3">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Driver</p>
                                <p class="mt-1 font-bold text-slate-800 dark:text-slate-200">{selectedNetwork.driver || "unknown"}</p>
                            </div>
                            <div class="rounded-xl bg-slate-50 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 p-3">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Scope</p>
                                <p class="mt-1 font-bold text-slate-800 dark:text-slate-200">{selectedNetwork.scope || "unknown"}</p>
                            </div>
                        </div>
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Flags</p>
                            <div class="flex flex-wrap gap-2">
                                <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase tracking-widest {selectedNetwork.internal ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}">Internal: {selectedNetwork.internal ? 'Yes' : 'No'}</span>
                                <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase tracking-widest {selectedNetwork.attachable ? 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}">Attachable: {selectedNetwork.attachable ? 'Yes' : 'No'}</span>
                            </div>
                        </div>
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">IPAM</p>
                            {#if selectedNetwork.ipam && selectedNetwork.ipam.length}
                                {#each selectedNetwork.ipam as block}
                                    <div class="text-[11px] text-slate-700 dark:text-slate-300">
                                        <p><span class="font-black uppercase text-[9px] tracking-widest text-slate-400">Subnet</span> <span class="font-mono">{block.subnet || "n/a"}</span></p>
                                        <p><span class="font-black uppercase text-[9px] tracking-widest text-slate-400">Gateway</span> <span class="font-mono">{block.gateway || "n/a"}</span></p>
                                    </div>
                                {/each}
                            {:else}
                                <p class="text-[11px] text-slate-500">No IPAM config reported.</p>
                            {/if}
                        </div>
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Attached Containers</p>
                            {#each positioned.edges.filter((e) => e.networkId === selectedNetwork.id) as e}
                                {@const c = visibleTopology.containers.find((x) => x.id === e.containerId)}
                                <button onclick={() => selectContainer(e.containerId)} class="w-full text-left rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/60 p-2 hover:border-cyan-400">
                                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200">{c?.name || e.endpointName || e.containerId}</p>
                                    <p class="text-[10px] text-slate-500 font-mono break-all">{e.ipv4Address || e.ipv6Address || e.macAddress || e.endpointId || "endpoint"}</p>
                                </button>
                            {/each}
                        </div>
                    </div>
                {:else if selectedContainer}
                    <div class="space-y-4">
                        <div class="rounded-2xl border border-cyan-200/70 dark:border-cyan-900/40 bg-cyan-50/70 dark:bg-cyan-900/10 p-4">
                            <p class="text-[10px] font-black uppercase tracking-widest text-cyan-700 dark:text-cyan-300">Container</p>
                            <p class="mt-1 text-lg font-black text-slate-900 dark:text-white break-all">{selectedContainer.name || selectedContainer.id.slice(0, 12)}</p>
                            <p class="mt-1 text-[10px] font-mono text-slate-500 break-all">{selectedContainer.id}</p>
                            {#if selectedContainer.image}
                                <p class="mt-2 text-[10px] font-mono text-slate-600 dark:text-slate-300 break-all">{selectedContainer.image}</p>
                            {/if}
                        </div>
                        <div class="grid grid-cols-2 gap-3 text-[11px]">
                            <div class="rounded-xl bg-slate-50 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 p-3">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">State</p>
                                <p class="mt-1 font-bold text-slate-800 dark:text-slate-200">{selectedContainer.state || "unknown"}</p>
                            </div>
                            <div class="rounded-xl bg-slate-50 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 p-3">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Networks</p>
                                <p class="mt-1 font-bold text-slate-800 dark:text-slate-200">{positioned.edges.filter((e) => e.containerId === selectedContainer.id).length}</p>
                            </div>
                        </div>
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-3 space-y-2">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Network Attachments</p>
                            {#each positioned.edges.filter((e) => e.containerId === selectedContainer.id) as e}
                                {@const n = visibleTopology.networks.find((x) => x.id === e.networkId)}
                                <button onclick={() => selectNetwork(e.networkId)} class="w-full text-left rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/60 p-2 hover:border-cyan-400">
                                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200">{n?.name || e.networkId}</p>
                                    <p class="text-[10px] text-slate-500 font-mono break-all">{edgeLabel(e)}</p>
                                </button>
                            {/each}
                        </div>
                    </div>
                {:else if selectedEdge}
                    <div class="space-y-4">
                        <div class="rounded-2xl border border-violet-200/70 dark:border-violet-900/40 bg-violet-50/70 dark:bg-violet-900/10 p-4">
                            <p class="text-[10px] font-black uppercase tracking-widest text-violet-700 dark:text-violet-300">Connection</p>
                            <p class="mt-1 text-sm font-black text-slate-900 dark:text-white">Endpoint Attachment</p>
                            <p class="mt-2 text-[10px] font-mono text-slate-600 dark:text-slate-300 break-all">{edgeLabel(selectedEdge)}</p>
                        </div>
                        <div class="space-y-2">
                            <button onclick={() => selectNetwork(selectedEdge.networkId)} class="w-full text-left rounded-xl border border-slate-200 dark:border-slate-700 p-3 hover:border-cyan-400">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Network</p>
                                <p class="text-sm font-bold text-slate-900 dark:text-white">{visibleTopology.networks.find((n) => n.id === selectedEdge.networkId)?.name || selectedEdge.networkId}</p>
                            </button>
                            <button onclick={() => selectContainer(selectedEdge.containerId)} class="w-full text-left rounded-xl border border-slate-200 dark:border-slate-700 p-3 hover:border-cyan-400">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Container</p>
                                <p class="text-sm font-bold text-slate-900 dark:text-white">{visibleTopology.containers.find((c) => c.id === selectedEdge.containerId)?.name || selectedEdge.containerId}</p>
                            </button>
                        </div>
                    </div>
                {:else}
                    <div class="space-y-4">
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 p-4">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Read-Only Topology Inspector</p>
                            <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">HarborWatch maps current Docker network attachments from runtime data. This view does not mutate Docker network configuration.</p>
                        </div>
                        <div class="grid grid-cols-3 gap-3">
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 p-3 bg-white dark:bg-slate-800/60">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Networks</p>
                                <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{visibleTopology.networks.length}</p>
                            </div>
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 p-3 bg-white dark:bg-slate-800/60">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Containers</p>
                                <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{visibleTopology.containers.length}</p>
                            </div>
                            <div class="rounded-xl border border-slate-200 dark:border-slate-700 p-3 bg-white dark:bg-slate-800/60">
                                <p class="text-[9px] font-black uppercase tracking-widest text-slate-400">Links</p>
                                <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{visibleTopology.edges.length}</p>
                            </div>
                        </div>
                        <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 p-4">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Usage</p>
                            <ul class="mt-2 space-y-2 text-sm text-slate-600 dark:text-slate-300">
                                <li>Click a network node to inspect driver, IPAM and attached containers.</li>
                                <li>Click a container node to inspect its network attachments and endpoint addresses.</li>
                                <li>Hover a link to see endpoint IP/MAC details inline.</li>
                            </ul>
                        </div>
                    </div>
                {/if}
                </aside>
            </div>
        </div>
    {/if}
</div>
