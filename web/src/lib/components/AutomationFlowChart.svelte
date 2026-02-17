<script lang="ts">
    import { chart } from "svelte-apexcharts";

    interface FlowStep {
        label: string;
        state?: "active" | "idle" | "warning";
    }

    let { title, subtitle = "", steps = [], accent = "#0ea5e9" } = $props<{
        title: string;
        subtitle?: string;
        steps: FlowStep[];
        accent?: string;
    }>();

    function markerColor(state?: string): string {
        switch (state) {
            case "active":
                return accent;
            case "warning":
                return "#f97316";
            default:
                return "#94a3b8";
        }
    }

    let series = $derived([
        {
            name: "Flow",
            data: steps.map((_s, idx) => ({
                x: idx + 1,
                y: 1
            }))
        }
    ]);

    let discrete = $derived(
        steps.map((step, idx) => ({
            seriesIndex: 0,
            dataPointIndex: idx,
            fillColor: markerColor(step.state),
            strokeColor: markerColor(step.state),
            size: 7
        }))
    );

    let options = $derived({
        series,
        chart: {
            type: "line",
            height: 220,
            toolbar: { show: false },
            zoom: { enabled: false },
            foreColor: "#64748b",
            background: "transparent"
        },
        stroke: {
            curve: "straight",
            width: 3,
            colors: [accent]
        },
        markers: {
            size: 5,
            discrete
        },
        dataLabels: {
            enabled: false
        },
        legend: {
            show: false
        },
        title: {
            text: title,
            align: "left",
            style: {
                fontSize: "12px",
                fontWeight: "900",
                fontFamily: "Montserrat",
                color: "#475569"
            }
        },
        subtitle: {
            text: subtitle,
            align: "left",
            style: {
                fontSize: "11px",
                fontWeight: "600",
                fontFamily: "Montserrat",
                color: "#94a3b8"
            }
        },
        xaxis: {
            categories: steps.map((s) => s.label),
            labels: {
                rotate: -25,
                trim: true,
                style: {
                    fontSize: "10px",
                    fontWeight: 700
                }
            },
            axisBorder: { show: false },
            axisTicks: { show: false }
        },
        yaxis: {
            min: 0,
            max: 2,
            labels: { show: false }
        },
        grid: {
            strokeDashArray: 5,
            borderColor: "#e2e8f0"
        },
        tooltip: {
            x: { show: true },
            y: { formatter: () => "Step" }
        }
    });
</script>

{#if steps.length > 0}
    {#key `${title}:${steps.length}:${steps.map((s) => s.state || "idle").join(",")}`}
        <div class="w-full" use:chart={options}></div>
    {/key}
{:else}
    <div class="h-[220px] rounded-xl border border-dashed border-slate-200 dark:border-slate-700 flex items-center justify-center text-xs italic text-slate-400">
        No flow steps configured.
    </div>
{/if}
