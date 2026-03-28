<script lang="ts">
    let { active = false } = $props<{ active?: boolean }>();

    let canvas: HTMLCanvasElement | null = $state(null);
    let rafId: number | null = null;
    let fadeTimer: ReturnType<typeof setTimeout> | null = null;
    let lastFrame = 0;
    let showing = $state(false);
    const FRAME_MS = 1000 / 12; // 12 fps — fast enough to feel alive, cheap enough for multiple cards
    const FADE_MS = 600;

    function drawFrame() {
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        const w = canvas.width;
        const h = canvas.height;
        const img = ctx.createImageData(w, h);
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const v = Math.random() > 0.5 ? 220 : 20;
            d[i] = d[i + 1] = d[i + 2] = v;
            d[i + 3] = 255;
        }
        ctx.putImageData(img, 0, 0);
    }

    // tick does not gate on `active` — it runs until cancelled so static
    // continues animating during the fade-out transition
    function tick(ts: number) {
        if (!canvas) return;
        if (ts - lastFrame >= FRAME_MS) {
            drawFrame();
            lastFrame = ts;
        }
        rafId = requestAnimationFrame(tick);
    }

    function stopRaf() {
        if (rafId !== null) { cancelAnimationFrame(rafId); rafId = null; }
    }

    $effect(() => {
        if (active && canvas) {
            // Cancel any pending fade-out stop
            if (fadeTimer !== null) { clearTimeout(fadeTimer); fadeTimer = null; }
            const parent = canvas.parentElement;
            canvas.width  = parent ? (parent.offsetWidth  || 480) : 480;
            canvas.height = parent ? (parent.offsetHeight || 240) : 240;
            lastFrame = 0;
            showing = true;
            if (rafId === null) rafId = requestAnimationFrame(tick);
        } else {
            // Fade out: CSS transition handles opacity, stop RAF after fade completes
            showing = false;
            if (fadeTimer !== null) clearTimeout(fadeTimer);
            fadeTimer = setTimeout(() => {
                stopRaf();
                if (canvas) {
                    const ctx = canvas.getContext('2d');
                    if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
                }
                fadeTimer = null;
            }, FADE_MS);
        }
        return () => {
            if (fadeTimer !== null) { clearTimeout(fadeTimer); fadeTimer = null; }
            stopRaf();
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
    class="absolute inset-0 h-full w-full pointer-events-none transition-opacity duration-500"
    style="opacity: {showing ? 0.20 : 0};"
></canvas>
