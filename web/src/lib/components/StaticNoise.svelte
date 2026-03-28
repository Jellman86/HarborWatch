<script lang="ts">
    let { active = false } = $props<{ active?: boolean }>();

    let canvas: HTMLCanvasElement | null = $state(null);
    let rafId: number | null = null;
    let lastFrame = 0;
    const FRAME_MS = 1000 / 12; // 12 fps — fast enough to feel alive, cheap enough for multiple cards

    function drawFrame() {
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        const w = canvas.width;
        const h = canvas.height;
        const img = ctx.createImageData(w, h);
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            // High-contrast dots: random choice between near-white and near-black
            // gives visible static on both light and dark card backgrounds
            const v = Math.random() > 0.5 ? 220 : 20;
            d[i] = d[i + 1] = d[i + 2] = v;
            d[i + 3] = 255;
        }
        ctx.putImageData(img, 0, 0);
    }

    function tick(ts: number) {
        if (!active || !canvas) return;
        if (ts - lastFrame >= FRAME_MS) {
            drawFrame();
            lastFrame = ts;
        }
        rafId = requestAnimationFrame(tick);
    }

    $effect(() => {
        if (active && canvas) {
            const parent = canvas.parentElement;
            canvas.width  = parent ? (parent.offsetWidth  || 480) : 480;
            canvas.height = parent ? (parent.offsetHeight || 240) : 240;
            lastFrame = 0;
            rafId = requestAnimationFrame(tick);
        } else {
            if (rafId !== null) {
                cancelAnimationFrame(rafId);
                rafId = null;
            }
            if (canvas) {
                const ctx = canvas.getContext('2d');
                if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
            }
        }
        return () => {
            if (rafId !== null) {
                cancelAnimationFrame(rafId);
                rafId = null;
            }
        };
    });
</script>

<!--
    Sits first in the DOM inside a `relative overflow-hidden` article,
    so all subsequent `relative` content naturally paints above it.
    pointer-events-none keeps clicks falling through to the card.
-->
<canvas
    bind:this={canvas}
    class="absolute inset-0 h-full w-full pointer-events-none"
    style="opacity: 0.20;"
></canvas>
