<script lang="ts">
    let { size = 16, class: className = "" } = $props<{ size?: number; class?: string }>();
</script>

<svg 
    width={size} 
    height={size} 
    viewBox="0 0 24 24" 
    fill="currentColor" 
    class={className}
    xmlns="http://www.w3.org/2000/svg"
>
    <!-- Stylized Portainer 'P' / Stack representation -->
    <path d="M12 2L4 7V17L12 22L20 17V7L12 2ZM12 4.18L18 7.93V10.34L12 6.59L6 10.34V7.93L12 4.18ZM6 12.66L12 8.91L18 12.66V15.07L12 11.32L6 15.07V12.66ZM12 19.82L6 16.07V13.66L12 17.41L18 13.66V16.07L12 19.82Z"/>
</svg>
