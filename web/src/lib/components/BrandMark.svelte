<script lang="ts">
    type LoadingMode = 'eager' | 'lazy';

    let {
        alt = 'HarborWatch',
        class: className = '',
        sizes = '80px',
        width,
        height,
        loading = 'eager',
    } = $props<{
        alt?: string;
        class?: string;
        sizes?: string;
        width?: number;
        height?: number;
        loading?: LoadingMode;
    }>();

    const srcSet = [
        '/logo-64.png 64w',
        '/logo-96.png 96w',
        '/logo-128.png 128w',
        '/logo-256.png 256w',
        '/logo-512.png 512w',
    ].join(', ');
</script>

<img
    src="/logo-128.png"
    srcset={srcSet}
    {sizes}
    {width}
    {height}
    {loading}
    decoding="async"
    alt={alt}
    class={`object-contain ${className}`.trim()}
/>
