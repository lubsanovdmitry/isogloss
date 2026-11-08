package history

import (
	"slices"

	"isogloss/engine/lex"
)

// Event is one entry in the chronicle.
type Event struct {
	Tick  int32
	Kind  string // "innovation", "split", "extinct", "intervention", "settle"
	Hex   int32
	Innov uint32
	Lang  int32
	Text  string
}

// InnovSet returns the sorted innovation IDs a variety adopted.
func InnovSet(v *lex.Variety) []uint32 {
	var out []uint32
	for n := v.Hist; n != nil; n = n.Parent {
		if n.E.Kind == lex.EntryInnov {
			out = append(out, n.E.Innov)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func intersect(a, b []uint32) []uint32 {
	var out []uint32
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}

func subtract(a, b []uint32) []uint32 {
	var out []uint32
	j := 0
	for _, x := range a {
		for j < len(b) && b[j] < x {
			j++
		}
		if j < len(b) && b[j] == x {
			continue
		}
		out = append(out, x)
	}
	return out
}

// TreeNode is a node of the family tree. Leaves carry a language ID; inner
// nodes are subgroups defined by the innovations all their members share.
type TreeNode struct {
	ID       int32
	Lang     int32 // -1 for inner nodes
	Children []int32
	Parent   int32
	Shared   []uint32 // innovations shared by every member
	Defining []uint32 // Shared minus the parent's Shared
}

// Leaf is one language's input to BuildTree.
type Leaf struct {
	Lang   int32
	Innovs []uint32
}

// BuildTree groups languages the way the comparative method does: by shared
// innovations. It repeatedly joins the two groups sharing the most
// innovations (ties broken by lowest IDs). Node 0 is the root (the
// proto-language); its Defining set is what every language shares.
func BuildTree(leaves []Leaf) []TreeNode {
	nodes := []TreeNode{{ID: 0, Lang: -1, Parent: -1}}
	var active []int32
	for _, l := range leaves {
		id := int32(len(nodes))
		nodes = append(nodes, TreeNode{ID: id, Lang: l.Lang, Parent: -1, Shared: l.Innovs})
		active = append(active, id)
	}
	for len(active) > 1 {
		bi, bj, best := 0, 1, -1
		for i := 0; i < len(active); i++ {
			for j := i + 1; j < len(active); j++ {
				n := len(intersect(nodes[active[i]].Shared, nodes[active[j]].Shared))
				if n > best {
					bi, bj, best = i, j, n
				}
			}
		}
		a, b := active[bi], active[bj]
		shared := intersect(nodes[a].Shared, nodes[b].Shared)
		if len(active) == 2 {
			// Final join: the root itself.
			nodes[0].Children = []int32{a, b}
			nodes[0].Shared = shared
			nodes[a].Parent, nodes[b].Parent = 0, 0
			active = nil
			break
		}
		id := int32(len(nodes))
		nodes = append(nodes, TreeNode{ID: id, Lang: -1, Parent: -1, Children: []int32{a, b}, Shared: shared})
		nodes[a].Parent, nodes[b].Parent = id, id
		active = append(active[:bj], active[bj+1:]...)
		active[bi] = id
	}
	if len(active) == 1 {
		nodes[0].Children = []int32{active[0]}
		nodes[active[0]].Parent = 0
	}
	// Collapse inner nodes that add no defining innovations into their parent
	// (a rake rather than an arbitrary binary resolution).
	for i := range nodes {
		p := nodes[i].Parent
		if p >= 0 {
			nodes[i].Defining = subtract(nodes[i].Shared, nodes[p].Shared)
		} else {
			nodes[i].Defining = nodes[i].Shared
		}
	}
	changed := true
	for changed {
		changed = false
		for i := 1; i < len(nodes); i++ {
			n := &nodes[i]
			if n.Lang >= 0 || n.Parent < 0 || len(n.Defining) > 0 || len(n.Children) == 0 {
				continue
			}
			p := &nodes[n.Parent]
			var kids []int32
			for _, c := range p.Children {
				if c == n.ID {
					kids = append(kids, n.Children...)
				} else {
					kids = append(kids, c)
				}
			}
			p.Children = kids
			for _, c := range n.Children {
				nodes[c].Parent = p.ID
			}
			n.Children, n.Parent = nil, -2 // detached
			changed = true
		}
	}
	return nodes
}
