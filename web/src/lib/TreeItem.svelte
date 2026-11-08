<script lang="ts">
  import type { Lang, TreeNode } from './engine';
  import { langColor } from './palette';
  import Self from './TreeItem.svelte';

  interface Props {
    node: TreeNode;
    nodes: Map<number, TreeNode>;
    langs: Lang[];
    proto: string;
    onlang: (id: number) => void;
    oninnov: (id: number) => void;
  }
  let { node, nodes, langs, proto, onlang, oninnov }: Props = $props();
  let open = $state(false);
  const lang = $derived(node.lang >= 0 ? langs[node.lang] : null);
  const name = $derived(node.id === 0 ? proto : lang ? lang.name : 'Subgroup');
  const n = $derived(node.defining.length);
</script>

<li>
  <div class="row">
    {#if lang}
      <span class="sw" style:background={langColor(lang.id)}></span>
      <button class="link name" onclick={() => onlang(lang.id)}>{name}</button>
      <span class="meta">{lang.size} {lang.size === 1 ? 'community' : 'communities'}</span>
    {:else}
      <span class="name" class:proto={node.id === 0}>{name}</span>
    {/if}
    {#if n > 0}
      <button class="link meta" aria-expanded={open} onclick={() => (open = !open)}>
        {n} defining {n === 1 ? 'change' : 'changes'}
      </button>
    {/if}
  </div>
  {#if open}
    <ul class="defs">
      {#each node.defining as d (d.id)}
        <li><button class="link" onclick={() => oninnov(d.id)}>{d.label}</button></li>
      {/each}
    </ul>
  {/if}
  {#if node.children.length}
    <ul class="kids">
      {#each node.children as c (c)}
        {@const child = nodes.get(c)}
        {#if child}<Self node={child} {nodes} {langs} {proto} {onlang} {oninnov} />{/if}
      {/each}
    </ul>
  {/if}
</li>

<style>
  li { list-style: none; }
  .row { display: flex; align-items: baseline; gap: 0.5rem; flex-wrap: wrap; padding: 0.15rem 0; }
  .kids { margin: 0; padding-left: 1.1rem; border-left: 1px solid var(--rule); margin-left: 0.3rem; }
  .defs { margin: 0.1rem 0 0.3rem 1.2rem; padding: 0; font-size: var(--t-sm); columns: 2; }
  .name { font-family: var(--serif); font-size: var(--t-lg); }
  .proto { font-style: italic; }
  .meta { color: var(--ink-soft); font-size: var(--t-sm); }
  .sw { width: 0.8rem; height: 0.8rem; border-radius: 2px; flex: none; align-self: center; }
  .link { background: none; border: none; padding: 0; text-align: left; cursor: pointer; }
  .link:hover { text-decoration: underline; }
</style>
