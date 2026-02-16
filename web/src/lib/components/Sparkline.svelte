<script lang="ts">
    import { onMount } from 'svelte';
    import { chart } from 'svelte-apexcharts';
    import type { Metric } from '../api-types';

    let { containerId } = $props<{ containerId: string }>();
    let metrics = $state<Metric[]>([]);
    let loading = $state(true);

    async function loadMetrics() {
        try {
            const res = await fetch(`/api/metrics/${containerId}?duration=1h`);
            if (res.ok) {
                const data = await res.json();
                metrics = data || [];
            }
        } catch {} finally {
            loading = false;
        }
    }

    onMount(() => {
        loadMetrics();
    });

    let series = $derived([
        {
            name: 'CPU',
            data: metrics.map(m => m.cpuPercent)
        }
    ]);

    let options = $derived({
        series: series,
        chart: {
            type: 'area',
            height: 30,
            width: 100,
            sparkline: { enabled: true },
            animations: { enabled: false }
        },
        stroke: {
            curve: 'smooth',
            width: 2,
            colors: ['#0ea5e9']
        },
        fill: {
            type: 'gradient',
            gradient: {
                shadeIntensity: 1,
                opacityFrom: 0.4,
                opacityTo: 0,
                stops: [0, 100]
            }
        },
        tooltip: { enabled: false }
    });
</script>

{#if !loading && metrics.length > 0}
    {#key metrics.length}
        <div class="w-[100px] h-[30px]" use:chart={options}></div>
    {/key}
{:else if loading}
    <div class="w-[100px] h-[30px] bg-slate-100 dark:bg-slate-800/50 rounded animate-pulse"></div>
{:else}
    <div class="w-[100px] h-[30px] flex items-center justify-center opacity-20 scale-75">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
    </div>
{/if}
