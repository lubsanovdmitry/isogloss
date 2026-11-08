<script lang="ts">
  // A Leipzig gloss: grammatical category labels (PL, 3SG, DEF) in small caps.
  let { text }: { text: string } = $props();
  const parts = $derived(text.split(/([-=.])/));
</script>

<span class="gloss">{#each parts as p, i (i)}{#if /^[0-9A-Z]+$/.test(p)}<span class="gc">{p.toLowerCase()}</span>{:else}{p}{/if}{/each}</span>

<style>
  .gc { font-variant-caps: small-caps; letter-spacing: 0.03em; }
</style>
