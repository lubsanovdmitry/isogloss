package sim

import (
	"maps"
	"slices"

	"isogloss/engine/history"
	"isogloss/engine/pop"
	"isogloss/engine/society"
)

// Clone returns an independent copy of the run: stepping either one never
// changes the other, and both continue exactly as the original would have
// (TestCloneContinuesExactly). The UI keeps clones every few generations
// so that looking back and forking do not replay from year 0.
//
// What is shared: the world, the proto-languages and substrates, the
// scenario, and every variety (varieties are persistent: a change makes a
// new one). Caches (the rule memo, the intelligibility cache) start empty
// in the clone; they only save work.
func (s *Sim) Clone() *Sim {
	c := *s
	// Innovations: struct copies (their adoption curves grow), remapped
	// wherever the run points at them.
	c.Reg = &history.Registry{All: make([]*history.Innovation, len(s.Reg.All))}
	for i, in := range s.Reg.All {
		cp := *in
		cp.Curve = slices.Clip(slices.Clone(in.Curve))
		c.Reg.All[i] = &cp
	}
	re := func(in *history.Innovation) *history.Innovation {
		if in == nil {
			return nil
		}
		return c.Reg.All[in.ID-1]
	}
	c.ap = history.NewApplier(c.Reg)
	c.cl = &pop.Clusterer{M: s.cl.M, Thr: s.cl.Thr}
	c.cl.Reset()

	c.Roots = slices.Clip(slices.Clone(s.Roots))
	c.ByHex = make([]*pop.Community, len(s.ByHex))
	for h, com := range s.ByHex {
		if com != nil {
			cp := *com
			cp.Diffusing = slices.Clip(slices.Clone(com.Diffusing))
			c.ByHex[h] = &cp
		}
	}
	c.Langs = make([]*Language, len(s.Langs))
	for i, l := range s.Langs {
		cp := *l
		cp.Members = slices.Clip(slices.Clone(l.Members))
		c.Langs[i] = &cp
	}
	c.Hist = make([]LangFrame, len(s.Hist))
	for i, f := range s.Hist {
		c.Hist[i] = LangFrame{Tick: f.Tick, Lang: slices.Clone(f.Lang)}
	}
	c.Tree = make([]history.TreeNode, len(s.Tree))
	for i, n := range s.Tree {
		n.Children = slices.Clone(n.Children)
		n.Shared = slices.Clone(n.Shared)
		n.Defining = slices.Clone(n.Defining)
		c.Tree[i] = n
	}
	c.Log = slices.Clip(slices.Clone(s.Log))
	c.Interventions = slices.Clip(slices.Clone(s.Interventions))
	c.Annotations = slices.Clip(slices.Clone(s.Annotations))

	c.live = make([]*liveInnov, len(s.live))
	for i, l := range s.live {
		w := *l.wave
		w.Level = slices.Clone(l.wave.Level)
		w.Applied = slices.Clone(l.wave.Applied)
		w.ResetScratch()
		c.live[i] = &liveInnov{in: re(l.in), wave: &w, age: l.age}
	}
	c.vars = maps.Clone(s.vars)
	c.present = slices.Clone(s.present)
	c.prestige = slices.Clone(s.prestige)
	c.contacts = make([][]pop.Contact, len(s.contacts))
	for i, k := range s.contacts {
		c.contacts[i] = slices.Clip(slices.Clone(k))
	}
	c.Subs = slices.Clone(s.Subs) // pointers: names are set once, at the start
	c.Region = slices.Clone(s.Region)
	c.Places = slices.Clone(s.Places)
	c.invented = slices.Clone(s.invented)
	c.subRoots = maps.Clone(s.subRoots)
	c.lang = slices.Clone(s.lang)

	c.Polities = make([]*society.Polity, len(s.Polities))
	for i, p := range s.Polities {
		cp := *p
		if p.Script != nil {
			o := *p.Script
			o.Phon = slices.Clone(o.Phon)
			o.Graph = slices.Clone(o.Graph)
			o.Spell = slices.Clone(o.Spell)
			o.Etym = slices.Clone(o.Etym)
			cp.Script = &o
		}
		c.Polities[i] = &cp
	}
	c.Routes = make([]society.Route, len(s.Routes))
	for i, r := range s.Routes {
		r.Path = slices.Clone(r.Path)
		c.Routes[i] = r
	}
	c.hub = maps.Clone(s.hub)
	c.routeEdge = maps.Clone(s.routeEdge)
	c.routeLinks = make(map[int32][]int32, len(s.routeLinks))
	for k, v := range s.routeLinks { //lint:ordered copy
		c.routeLinks[k] = slices.Clone(v)
	}
	c.conservative = slices.Clip(slices.Clone(s.conservative))
	if s.Camp != nil {
		cp := *s.Camp
		cp.steer = maps.Clone(s.Camp.steer)
		c.Camp = &cp
	}
	c.FixedRoutes = slices.Clip(slices.Clone(s.FixedRoutes))
	c.normOf = slices.Clone(s.normOf)
	c.levels = make(map[[4]int32]*history.Innovation, len(s.levels))
	for k, in := range s.levels { //lint:ordered copy
		c.levels[k] = re(in)
	}
	return &c
}
