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
//	query    {kind, ...}                 compare | lang | word | innov | innovs | preview | frames | concepts | sketch
//	save                                 returns the save (seed, tick, interventions)
//	load     {save}                      replay a save
//	fork     {tick}                      replay this run truncated at tick
//	bench    {seed, ticks}               time a fresh run (ms)
package main

import (
	"strconv"
	"syscall/js"
	"time"

	"isogloss/engine/lex"
	"isogloss/engine/render"
	"isogloss/engine/sim"
)

var cur *sim.Sim

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
		KI("tick", int64(sv.Tick)).KI("threshold", int64(sv.Config.Threshold)).K("interventions").Arr()
	for _, iv := range sv.Interventions {
		j.Obj().KI("tick", int64(iv.Tick)).KS("kind", iv.Kind).KI("hex", int64(iv.Hex)).KS("rule", iv.Rule).End()
	}
	j.EndArr().End()
	return j.String()
}

func parseSave(v js.Value) (sim.Save, bool) {
	seed, err := strconv.ParseUint(str(v, "seed"), 10, 64)
	if err != nil {
		return sim.Save{}, false
	}
	sv := sim.Save{Version: 1, Seed: seed, Config: sim.DefaultConfig(), Tick: int32(num(v, "tick", 0))}
	sv.Config.Threshold = uint32(num(v, "threshold", 0))
	ivs := v.Get("interventions")
	if ivs.Type() == js.TypeObject {
		for i := 0; i < ivs.Length(); i++ {
			x := ivs.Index(i)
			sv.Interventions = append(sv.Interventions, sim.Intervention{
				Tick: int32(num(x, "tick", 0)), Kind: str(x, "kind"), Hex: int32(num(x, "hex", -1)), Rule: str(x, "rule"),
			})
		}
	}
	return sv, true
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
		cur = sim.New(seed, cfg)
		return withWorld(cur, 0)
	case "step":
		n := num(cmd, "n", 1)
		for i := 0; i < n; i++ {
			cur.Step()
		}
		return string(render.SnapshotJSON(cur, num(cmd, "since", 0)))
	case "snapshot":
		return string(render.SnapshotJSON(cur, num(cmd, "since", 0)))
	case "intervene":
		iv := sim.Intervention{Kind: "rule", Hex: int32(num(cmd, "hex", -1)), Rule: str(cmd, "rule")}
		if err := cur.Intervene(iv); err != nil {
			return errJSON(err.Error())
		}
		return `{"ok":true,"tick":` + strconv.Itoa(int(cur.Tick+1)) + `}`
	case "save":
		return saveJSON(cur.SaveState())
	case "load":
		sv, ok := parseSave(cmd.Get("save"))
		if !ok {
			return errJSON("bad save")
		}
		cur = sim.Replay(sv)
		return withWorld(cur, 0)
	case "fork":
		cur = sim.Replay(cur.Fork(int32(num(cmd, "tick", int(cur.Tick)))))
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
		return query(cmd)
	}
	return errJSON("unknown command")
}

func query(q js.Value) string {
	kind := str(q, "kind")
	if kind == "concepts" {
		return string(render.ConceptsJSON())
	}
	if cur == nil {
		return errJSON("no run")
	}
	switch kind {
	case "compare":
		return string(render.Compare(cur, q.Get("core").Truthy()).JSON())
	case "lang":
		return string(render.LangJSON(cur, int32(num(q, "id", -1))))
	case "word":
		c := lex.ConceptID(num(q, "concept", 0))
		if l := str(q, "label"); l != "" {
			if id, ok := lex.ConceptByLabel(l); ok {
				c = id
			}
		}
		e, ok := render.EtymOf(cur, int32(num(q, "hex", -1)), c)
		if !ok {
			return errJSON("no such word")
		}
		return string(e.JSON())
	case "innov":
		return string(render.InnovJSON(cur, uint32(num(q, "id", 0))))
	case "innovs":
		return string(render.InnovListJSON(cur, num(q, "min", 3)))
	case "preview":
		return string(render.PreviewJSON(cur, int32(num(q, "hex", -1)), str(q, "rule")))
	case "frames":
		return string(render.FramesJSON(cur))
	case "save":
		return saveJSON(cur.SaveState())
	}
	return errJSON("unknown query " + kind)
}

func main() {
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
