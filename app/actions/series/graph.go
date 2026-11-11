/*
|--------------------------------------------------------------------------
| Series Graph
|--------------------------------------------------------------------------
|
| The steps of a series and the paths between them, looked up by id. A
| step without a path to follow ends the journey there: the person
| finished. Paths never lead back to a step already taken, so every
| journey ends.
|
*/

package series

import (
	"slices"
)

/*
graph indexes a spec's steps by id and their paths by step and path.
*/
type graph struct {
	spec  Spec
	nodes map[string]Node
	next  map[string]map[string]string
}

/*
compile indexes a spec.
*/
func compile(spec Spec) graph {
	g := graph{spec: spec, nodes: map[string]Node{}, next: map[string]map[string]string{}}
	for _, n := range spec.Nodes {
		g.nodes[n.ID] = n
	}

	for _, e := range spec.Edges {
		if g.next[e.From] == nil {
			g.next[e.From] = map[string]string{}
		}

		g.next[e.From][pathOrNext(e.Path)] = e.To
	}

	return g
}

/*
node returns a step by id.
*/
func (g graph) node(id string) (Node, bool) {
	n, ok := g.nodes[id]

	return n, ok
}

/*
target returns the step a path of a step leads to.
*/
func (g graph) target(from, path string) (string, bool) {
	to, ok := g.next[from][pathOrNext(path)]

	return to, ok
}

/*
hasPath reports whether a step's path leads anywhere.
*/
func (g graph) hasPath(from, path string) bool {
	_, ok := g.target(from, path)

	return ok
}

/*
cyclic reports whether a path leads back to a step already taken.
*/
func (g graph) cyclic() bool {
	state := map[string]int{}

	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}

		state[id] = 1
		for _, to := range g.next[id] {
			if visit(to) {
				return true
			}
		}

		state[id] = 2

		return false
	}

	for _, n := range g.spec.Nodes {
		if visit(n.ID) {
			return true
		}
	}

	return false
}

/*
reachable lists the steps a journey can reach from the entry.
*/
func (g graph) reachable() map[string]bool {
	seen := map[string]bool{}
	queue := []string{KindEntry}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if seen[id] {
			continue
		}

		seen[id] = true
		for _, to := range g.next[id] {
			queue = append(queue, to)
		}
	}

	return seen
}

/*
successor returns the first step after id that the other graph also
has, following each step's paths in a fixed order, for people waiting
on a step a new version removed.
*/
func (g graph) successor(id string, other graph) (string, bool) {
	seen := map[string]bool{}
	queue := []string{id}

	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]

		if seen[at] {
			continue
		}

		seen[at] = true
		if _, ok := other.nodes[at]; ok && at != id {
			return at, true
		}

		paths := make([]string, 0, len(g.next[at]))
		for p := range g.next[at] {
			paths = append(paths, p)
		}

		slices.Sort(paths)

		for _, p := range paths {
			queue = append(queue, g.next[at][p])
		}
	}

	return "", false
}

/*
messageNodes lists the steps that send a message.
*/
func (g graph) messageNodes() []Node {
	var out []Node

	for _, n := range g.spec.Nodes {
		if IsMessage(n.Kind) {
			out = append(out, n)
		}
	}

	return out
}

/*
pathOrNext names the default path when a path is left empty.
*/
func pathOrNext(path string) string {
	if path == "" {
		return PathNext
	}

	return path
}
