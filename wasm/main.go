//go:build js && wasm

// Command wasm is the browser bridge: it runs the engine inside a Web Worker
// and exposes one function, isoglossHandle(cmd) → JSON string.
//
// Commands (plain JS objects, field "cmd"):
//
//	init     {seed, threshold?}          new run; returns world + snapshot
//	step     {n, since}                  advance n ticks; returns snapshot
//	snapshot {since}
//	intervene{rule, hex}                 schedule a rule for the next tick
//	query    {kind, ...}                 compare | corresp | intel | lang | sketch | paradigm | cell | fable | word | innov | innovs | preview | suggest | structure | frames | concepts | labels | place
//	save                                 returns the save (seed, tick, interventions)
//	load     {save}                      replay a save
//	view     {tick}                      look at the run as it was at tick (-1: live); queries follow
//	fork     {tick}                      continue from tick, dropping later player actions
//	bench    {seed, ticks}               time a fresh run (ms)
package main

import (
	"runtime/debug"
	"strconv"
	"syscall/js"
	"time"

	"isogloss/engine/lex"
	"isogloss/engine/render"
	"isogloss/engine/scenario"
	"isogloss/engine/sim"
)

var (
	cur  *sim.Sim
	tl   *sim.Timeline // snapshots of cur for looking back and forking
	view *sim.Sim      // the past moment being looked at, nil for live
)

// setRun makes s the live run with a fresh timeline.
func setRun(s *sim.Sim, t *sim.Timeline) {
	cur, view = s, nil
	if t == nil {
		t = sim.NewTimeline(s)
	}
	tl = t
}

func errJSON(msg string) string {
	var j render.JSON
	return j.Obj().KS("error", msg).End().String()
}

func num(v js.Value, k string, def int) int {
	x := v.Get(k)
	if x.IsUndefined() || x.IsNull() {
		return def
	}
	return x.Int()
}

func str(v js.Value, k string) string {
	x := v.Get(k)
	if x.Type() != js.TypeString {
		return ""
	}
	return x.String()
}

func saveJSON(sv sim.Save) string {
	var j render.JSON
	j.Obj().KI("version", int64(sv.Version)).KS("seed", strconv.FormatUint(sv.Seed, 10)).
		KI("tick", int64(sv.Tick)).KI("threshold", int64(sv.Config.Threshold)).KS("scenario", sv.Scenario).K("interventions").Arr()
	for _, iv := range sv.Interventions {
		j.Obj().KI("tick", int64(iv.Tick)).KS("kind", iv.Kind).KI("hex", int64(iv.Hex)).KI("from", int64(iv.From)).
			KI("polity", int64(iv.Polity)).KS("rule", iv.Rule).KU("innov", uint64(iv.Innov)).KB("paid", iv.Paid).KB("scripted", iv.Scripted).End()
	}
	j.EndArr().K("annotations").Arr()
	for _, a := range sv.Annotations {
		j.Obj().KS("key", a.Key).KS("value", a.Value).End()
	}
	j.EndArr().End()
	return j.String()
}

func parseSave(v js.Value) (sim.Save, bool) {
	seed, err := strconv.ParseUint(str(v, "seed"), 10, 64)
	if err != nil {
		return sim.Save{}, false
	}
	sv := sim.Save{Version: 1, Seed: seed, Config: sim.DefaultConfig(), Tick: int32(num(v, "tick", 0)), Scenario: str(v, "scenario")}
	sv.Config.Threshold = uint32(num(v, "threshold", 0))
	ivs := v.Get("interventions")
	if ivs.Type() == js.TypeObject {
		for i := 0; i < ivs.Length(); i++ {
			x := ivs.Index(i)
			sv.Interventions = append(sv.Interventions, sim.Intervention{
				Tick: int32(num(x, "tick", 0)), Kind: str(x, "kind"), Hex: int32(num(x, "hex", -1)), Rule: str(x, "rule"),
				From: int32(num(x, "from", -1)), Polity: int32(num(x, "polity", -1)),
				Innov: uint32(num(x, "innov", 0)), Paid: x.Get("paid").Truthy(), Scripted: x.Get("scripted").Truthy(),
			})
		}
	}
	if as := v.Get("annotations"); as.Type() == js.TypeObject {
		for i := 0; i < as.Length(); i++ {
			sv.Annotations = append(sv.Annotations, sim.Annotation{Key: str(as.Index(i), "key"), Value: str(as.Index(i), "value")})
		}
	}
	return sv, true
}

// npSpec reads a noun phrase of an abstract text from JS.
func npSpec(v js.Value) *render.NPSpec {
	if v.Type() != js.TypeObject {
		return nil
	}
	n := &render.NPSpec{Noun: str(v, "noun"), Pron: str(v, "pron"), Adj: str(v, "adj"), PL: v.Get("pl").Truthy(), Def: v.Get("def").Truthy()}
	n.Gen = npSpec(v.Get("gen"))
	return n
}

// clauseSpecs reads an abstract text (a list of clauses) from JS.
func clauseSpecs(v js.Value) []render.ClauseSpec {
	var out []render.ClauseSpec
	if v.Type() != js.TypeObject {
		return nil
	}
	for i := 0; i < v.Length(); i++ {
		x := v.Index(i)
		c := render.ClauseSpec{Verb: str(x, "verb"), Tense: str(x, "tense"), English: str(x, "english"),
			Obj: npSpec(x.Get("obj")), Dat: npSpec(x.Get("dat")), Loc: npSpec(x.Get("loc"))}
		if s := npSpec(x.Get("subj")); s != nil {
			c.Subj = *s
		}
		out = append(out, c)
	}
	return out
}

// parseSeed accepts the seed as a decimal string (full 64-bit range) or a number.
func parseSeed(cmd js.Value) (uint64, string) {
	v := cmd.Get("seed")
	switch v.Type() {
	case js.TypeNumber:
		return uint64(v.Int()), ""
	case js.TypeString:
		if n, err := strconv.ParseUint(v.String(), 10, 64); err == nil {
			return n, ""
		}
	case js.TypeUndefined:
		return 42, ""
	}
	return 0, "the seed must be a whole number"
}

func withWorld(s *sim.Sim, since int) string {
	return `{"world":` + string(render.WorldJSON(s)) + `,"snapshot":` + string(render.SnapshotJSON(s, since)) + `}`
}

func handle(cmd js.Value) string {
	if cmd.Get("cmd").String() != "init" && cmd.Get("cmd").String() != "load" &&
		cmd.Get("cmd").String() != "bench" && cmd.Get("cmd").String() != "query" && cur == nil {
		return errJSON("no run; send init first")
	}
	switch cmd.Get("cmd").String() {
	case "init":
		seed, err := parseSeed(cmd)
		if err != "" {
			return errJSON(err)
		}
		cfg := sim.DefaultConfig()
		cfg.Threshold = uint32(num(cmd, "threshold", 0))
		var sc *scenario.Scenario
		if id := str(cmd, "scenario"); id != "" {
			src := str(cmd, "source")
			if st, ok := scenario.StarterByID(id); ok && src == "" {
				src = st.Src
			}
			var e error
			if sc, e = scenario.Read(src); e != nil {
				return errJSON(e.Error())
			}
		}
		s, e := sim.NewScenario(seed, cfg, sc)
		if e != nil {
			return errJSON(e.Error())
		}
		setRun(s, nil)
		return withWorld(cur, 0)
	case "step":
		view = nil
		n := num(cmd, "n", 1)
		for i := 0; i < n; i++ {
			cur.Step()
			tl.Record(cur)
		}
		return string(render.SnapshotJSON(cur, num(cmd, "since", 0)))
	case "snapshot":
		return string(render.SnapshotJSON(cur, num(cmd, "since", 0)))
	case "intervene":
		kind := str(cmd, "kind")
		if kind == "" {
			kind = "rule"
		}
		iv := sim.Intervention{Kind: kind, Hex: int32(num(cmd, "hex", -1)), From: int32(num(cmd, "from", -1)),
			Polity: int32(num(cmd, "polity", -1)), Rule: str(cmd, "rule"), Innov: uint32(num(cmd, "innov", 0))}
		if err := cur.Intervene(iv); err != nil {
			return errJSON(err.Error())
		}
		return `{"ok":true,"tick":` + strconv.Itoa(int(cur.Tick+1)) + `}`
	case "save":
		return saveJSON(cur.SaveState())
	case "annotate":
		cur.Annotate(str(cmd, "key"), str(cmd, "value"))
		return `{"ok":true}`
	case "load":
		sv, ok := parseSave(cmd.Get("save"))
		if !ok {
			return errJSON("bad save")
		}
		s, t, err := sim.ReplayTimeline(sv)
		if err != nil {
			return errJSON(err.Error())
		}
		setRun(s, t)
		return withWorld(cur, 0)
	case "view":
		// Look at the run as it was at tick t (t < 0 or ≥ now: live).
		t := int32(num(cmd, "tick", -1))
		if t < 0 || t >= cur.Tick {
			view = nil
			return string(render.SnapshotJSON(cur, 0))
		}
		if view == nil || view.Tick != t {
			view = tl.At(cur, t)
		}
		return string(render.SnapshotJSON(view, 0))
	case "fork":
		t := int32(num(cmd, "tick", int(cur.Tick)))
		f := view
		if f == nil || f.Tick != t {
			f = tl.At(cur, t)
		}
		tl.Truncate(t)
		cur, view = f, nil
		return withWorld(cur, 0)
	case "bench":
		seed, err := parseSeed(cmd)
		if err != "" {
			return errJSON(err)
		}
		start := time.Now()
		s := sim.New(seed, sim.DefaultConfig())
		s.Run(num(cmd, "ticks", 40))
		ms := time.Since(start).Milliseconds()
		return `{"ms":` + strconv.FormatInt(ms, 10) + `,"languages":` + strconv.Itoa(len(s.LivingLanguages())) + `}`
	case "query":
		if view != nil {
			return query(cmd, view)
		}
		return query(cmd, cur)
	}
	return errJSON("unknown command")
}

// query answers a view query against s: the live run, or the past
// moment being looked at (frames, annotations and saves always come from
// the live run).
func query(q js.Value, s *sim.Sim) string {
	kind := str(q, "kind")
	if kind == "concepts" {
		return string(render.ConceptsJSON())
	}
	if kind == "cells" {
		return string(render.CellsJSON())
	}
	if kind == "scenarios" {
		return string(render.ScenariosJSON())
	}
	if s == nil {
		return errJSON("no run")
	}
	switch kind {
	case "compare":
		return string(render.Compare(s, q.Get("core").Truthy()).JSON(s))
	case "lang":
		return string(render.LangJSON(s, int32(num(q, "id", -1))))
	case "word":
		c := lex.ConceptID(num(q, "concept", 0))
		if l := str(q, "label"); l != "" {
			if id, ok := lex.ConceptByLabel(l); ok {
				c = id
			}
		}
		e, ok := render.EtymOf(s, int32(num(q, "hex", -1)), c)
		if !ok {
			return errJSON("no such word")
		}
		return string(e.JSON())
	case "innov":
		return string(render.InnovJSON(s, uint32(num(q, "id", 0))))
	case "innovs":
		return string(render.InnovListJSON(s, num(q, "min", 3)))
	case "structure":
		return string(render.StructureJSON(s, int32(num(q, "hex", -1))))
	case "suggest":
		return string(render.SuggestJSON(s, int32(num(q, "hex", -1)), num(q, "n", 8), uint64(num(q, "salt", 0))))
	case "preview":
		return string(render.PreviewJSON(s, int32(num(q, "hex", -1)), str(q, "rule")))
	case "sketch":
		return string(render.SketchJSON(s, int32(num(q, "id", -1))))
	case "paradigm":
		c, _ := lex.ConceptByLabel(str(q, "label"))
		return string(render.ParadigmJSON(s, int32(num(q, "lang", -1)), c))
	case "cell":
		c, ok := lex.ConceptByLabel(str(q, "label"))
		if !ok {
			return errJSON("no such word")
		}
		return string(render.CellEtymJSON(s, int32(num(q, "hex", -1)), c, num(q, "cell", 0)))
	case "fable":
		return string(render.FableJSON(s))
	case "corresp":
		return string(render.CorrespondencesJSON(s, num(q, "min", 2)))
	case "intel":
		return string(render.IntelJSON(s))
	case "frames":
		return string(render.FramesJSON(cur))
	case "namecands":
		id := int32(num(q, "id", -1))
		if str(q, "of") == "polity" { // not "kind": that names the query
			return string(render.CandidatesJSON(render.PolityNameCandidates(s, id)))
		}
		return string(render.CandidatesJSON(render.LangNameCandidates(s, id)))
	case "era":
		return string(render.EraJSON(s, int32(num(q, "from", 0)), int32(num(q, "to", int(s.Tick)+1))))
	case "mythspec":
		return string(render.MythSpecJSON())
	case "rendertext":
		cl, err := render.BuildClauses(clauseSpecs(q.Get("clauses")))
		if err != nil {
			return errJSON(err.Error())
		}
		return string(render.TextJSON(s, cl))
	case "annotations":
		var j render.JSON
		j.Arr()
		for _, a := range cur.Annotations {
			j.Obj().KS("key", a.Key).KS("value", a.Value).End()
		}
		j.EndArr()
		return j.String()
	case "campaign":
		return string(render.CampaignJSON(s, true))
	case "labels":
		return string(render.LabelsJSON(s, num(q, "max", 14)))
	case "place":
		return string(render.PlaceJSON(s, int32(num(q, "hex", -1))))
	case "save":
		return saveJSON(cur.SaveState())
	}
	return errJSON("unknown query " + kind)
}

func main() {
	// Collect garbage half as often (12% faster runs), within a soft cap:
	// wasm memory never shrinks, and the timeline keeps snapshots alive.
	debug.SetGCPercent(200)
	debug.SetMemoryLimit(1 << 30)
	js.Global().Set("isoglossHandle", js.FuncOf(func(this js.Value, args []js.Value) (ret any) {
		defer func() {
			// Surface engine panics to the UI instead of killing the worker.
			if r := recover(); r != nil {
				msg := "engine panic"
				switch x := r.(type) {
				case error:
					msg += ": " + x.Error()
				case string:
					msg += ": " + x
				}
				ret = errJSON(msg)
			}
		}()
		if len(args) == 0 {
			return errJSON("missing command")
		}
		return handle(args[0])
	}))
	js.Global().Call("postMessage", map[string]any{"ready": true})
	select {}
}
