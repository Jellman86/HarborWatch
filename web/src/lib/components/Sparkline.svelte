<script lang="ts">
    import type { Metric } from "../api-types";

    let { metrics = [] } = $props<{ metrics?: Metric[] }>();

    const WIDTH = 100;
    const HEIGHT = 30;
    const PAD = 2;

    const toFinite = (value: unknown): number | null => {
        const n = Number(value);
        return Number.isFinite(n) ? n : null;
    };

    let safeData = $derived(
        (metrics || [])
            .map((m) => toFinite(m?.cpuPercent))
            .filter((v): v is number => v !== null)
            .map((v) => Math.max(0, v))
    );

    let bounds = $derived((() => {
        if (safeData.length === 0) return { min: 0, max: 1 };
        const min = safeData.reduce((a, b) => (a < b ? a : b), safeData[0]);
        const max = safeData.reduce((a, b) => (a > b ? a : b), safeData[0]);
        return {
            min,
            max: Math.max(max, min + 1)
        };
    })());

    let points = $derived(safeData.map((value, idx) => {
        const x = safeData.length > 1
            ? PAD + (idx / (safeData.length - 1)) * (WIDTH - PAD * 2)
            : WIDTH / 2;
        const yRatio = (value - bounds.min) / (bounds.max - bounds.min);
        const y = HEIGHT - PAD - yRatio * (HEIGHT - PAD * 2);
        return { x, y };
    }));

    let linePath = $derived(
        points.length > 0
            ? points.map((p, idx) => `${idx === 0 ? "M" : "L"} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(" ")
            : ""
    );

    let areaPath = $derived((() => {
        if (points.length === 0) return "";
        const first = points[0];
        const last = points[points.length - 1];
        return `${linePath} L ${last.x.toFixed(2)} ${(HEIGHT - PAD).toFixed(2)} L ${first.x.toFixed(2)} ${(HEIGHT - PAD).toFixed(2)} Z`;
    })());
</script>

{#if points.length > 0}
    <svg class="w-[100px] h-[30px]" viewBox={`0 0 ${WIDTH} ${HEIGHT}`} preserveAspectRatio="none" role="img" aria-label="CPU sparkline">
        <path d={areaPath} fill="#0ea5e933" />
        <path d={linePath} fill="none" stroke="#0ea5e9" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
{:else}
    <div class="w-[100px] h-[30px] flex items-center justify-center opacity-20 scale-75">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
    </div>
{/if}
