<script lang="ts">
    import Chart from 'svelte-apexcharts';
    import type { Metric } from '../api-types';

    let { metrics, title, type = 'cpu' } = $props<{
        metrics: Metric[];
        title: string;
        type: 'cpu' | 'memory';
    }>();

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    let series = $derived([
        {
            name: type === 'cpu' ? 'CPU %' : 'Memory Usage',
            data: metrics.map(m => ({
                x: m.timestamp * 1000,
                y: type === 'cpu' ? m.cpuPercent : m.memoryUsage
            }))
        }
    ]);

    let options = $derived({
        chart: {
            type: 'area',
            height: 250,
            animations: { enabled: true },
            toolbar: { show: false },
            zoom: { enabled: false },
            background: 'transparent',
            foreColor: '#94a3b8'
        },
        title: {
            text: title,
            align: 'left',
            style: {
                fontSize: '12px',
                fontWeight: '900',
                fontFamily: 'Montserrat',
                color: '#64748b'
            }
        },
        stroke: {
            curve: 'smooth',
            width: 2,
            colors: [type === 'cpu' ? '#0ea5e9' : '#8b5cf6']
        },
        fill: {
            type: 'gradient',
            gradient: {
                shadeIntensity: 1,
                opacityFrom: 0.45,
                opacityTo: 0.05,
                stops: [20, 100],
                colorStops: [
                    {
                        offset: 0,
                        color: type === 'cpu' ? '#0ea5e9' : '#8b5cf6',
                        opacity: 0.4
                    },
                    {
                        offset: 100,
                        color: type === 'cpu' ? '#0ea5e9' : '#8b5cf6',
                        opacity: 0
                    }
                ]
            }
        },
        xaxis: {
            type: 'datetime',
            labels: {
                datetimeUTC: false,
                style: { fontSize: '10px' }
            },
            axisBorder: { show: false },
            axisTicks: { show: false }
        },
        yaxis: {
            labels: {
                style: { fontSize: '10px' },
                formatter: (val: number) => type === 'cpu' ? val.toFixed(1) + '%' : formatBytes(val)
            }
        },
        grid: {
            borderColor: '#334155',
            strokeDashArray: 4,
            xaxis: { lines: { show: true } },
            yaxis: { lines: { show: true } }
        },
        theme: {
            mode: 'dark'
        },
        tooltip: {
            x: { format: 'dd MMM HH:mm' },
            theme: 'dark'
        }
    });
</script>

<div class="w-full">
    <Chart {options} {series} />
</div>
