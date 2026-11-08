// Muted lithograph tints for languages, in the manner of hand-coloured
// dialect atlases. Indexed by language ID.
const light = ['#8fb39a', '#d29a6a', '#86a3cf', '#c4959e', '#d9bf6a', '#6f95a3', '#ad86aa', '#a3bf72', '#c98a7a', '#62a3a3', '#b8a37d', '#9488c0'];
const dark = ['#5f8a6c', '#a8714a', '#5b77a3', '#946a72', '#a99247', '#4f6f7c', '#80607f', '#76904c', '#9a6153', '#437777', '#8a7856', '#6b609a'];

const prefersDark = typeof matchMedia !== 'undefined' && matchMedia('(prefers-color-scheme: dark)').matches;

export function langColor(id: number): string {
  if (id < 0) return 'transparent';
  const p = prefersDark ? dark : light;
  return p[id % p.length];
}

// Pale biome tints for unsettled land.
const biomeLight: Record<string, string> = {
  coast: '#e3e6dc', plains: '#e2e4d4', forest: '#d6ddcd', steppe: '#e7e3d0',
  desert: '#ede5cf', mountain: '#d4d2cc', island: '#e3e6dc',
};
const biomeDark: Record<string, string> = {
  coast: '#283043', plains: '#2a3142', forest: '#243042', steppe: '#2e3142',
  desert: '#333243', mountain: '#33394a', island: '#283043',
};

export function biomeColor(b: string): string {
  return (prefersDark ? biomeDark : biomeLight)[b] ?? 'transparent';
}

// Cognate-class tints for the comparative table (class 0 = inherited root).
export function cogTint(k: number): string {
  if (k === 0) return 'transparent';
  const hues = [28, 200, 300, 120, 50, 250, 340, 170];
  const h = hues[(k - 1) % hues.length];
  return prefersDark ? `hsl(${h} 30% 26%)` : `hsl(${h} 45% 88%)`;
}
