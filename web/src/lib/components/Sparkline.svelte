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
                metrics = await res.json();
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
    <div class="w-[100px] h-[30px]" use:chart={options}></div>
{:else}
    <div class="w-[100px] h-[30px] bg-slate-100 dark:bg-slate-800/50 rounded animate-pulse"></div>
{/if}
