package complete

import (
	"reflect"

	"src.elv.sh/pkg/parse"
)

type matcher interface {
	match([]parse.Node) ([]parse.Node, bool)
}

// Reports whether a and b have the same dynamic type. Useful as a more succinct
// alternative to type assertions.
func is(a, b parse.Node) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}

type nodePath []parse.Node

func (ns nodePath) match(ms ...matcher) bool {
	for _, m := range ms {
		ns2, ok := m.match(ns)
		if !ok {
			return false
		}
		ns = ns2
	}
	return true
}

// Returns the path of Node's from n to a leaf at position p. Leaf first in the
// returned slice.
func findNodePath(root parse.Node, p int) nodePath {
	n := root
descend:
	for len(parse.Children(n)) > 0 {
		for _, ch := range parse.Children(n) {
			if rg := ch.Range(); rg.From <= p && p <= rg.To {
				n = ch
				continue descend
			}
		}
		return nil
	}
	var path []parse.Node
	for {
		path = append(path, n)
		if n == root {
			break
		}
		n = parse.Parent(n)
	}
	return path
}

// TODO: Avoid reflection with generics.
type typeMatcher struct{ typ reflect.Type }

func typed(n parse.Node) matcher { return typeMatcher{reflect.TypeOf(n)} }

func (m typeMatcher) match(ns []parse.Node) ([]parse.Node, bool) {
	if len(ns) > 0 && reflect.TypeOf(ns[0]) == m.typ {
		return ns[1:], true
	}
	return nil, false
}

var (
	aChunk    = typed(&parse.Chunk{})
	aPipeline = typed(&parse.Pipeline{})
	aForm     = typed(&parse.Form{})
	aArray    = typed(&parse.Array{})
	aCompound = typed(&parse.Compound{})
	aIndexing = typed(&parse.Indexing{})
	aPrimary  = typed(&parse.Primary{})
	aRedir    = typed(&parse.Redir{})
	aSep      = typed(&parse.Sep{})
)

type storeMatcher struct {
	p   reflect.Value
	typ reflect.Type
}

func store(p interface{}) matcher {
	dst := reflect.ValueOf(p).Elem()
	return storeMatcher{dst, dst.Type()}
}

func (m storeMatcher) match(ns []parse.Node) ([]parse.Node, bool) {
	if len(ns) > 0 && reflect.TypeOf(ns[0]) == m.typ {
		m.p.Set(reflect.ValueOf(ns[0]))
		return ns[1:], true
	}
	return nil, false
}

type simplePrimaryMatcher struct {
	ev       PureEvaler
	s        string
	compound *parse.Compound
	primary  *parse.Primary
}

func simplePrimaryExpr(ev PureEvaler) *simplePrimaryMatcher {
	return &simplePrimaryMatcher{ev: ev}
}

func (m *simplePrimaryMatcher) match(ns []parse.Node) ([]parse.Node, bool) {
	if len(ns) < 3 {
		return nil, false
	}
	primary, ok := ns[0].(*parse.Primary)
	if !ok {
		return nil, false
	}
	indexing, ok := ns[1].(*parse.Indexing)
	if !ok {
		return nil, false
	}
	compound, ok := ns[2].(*parse.Compound)
	if !ok {
		return nil, false
	}
	s, ok := m.ev.PurelyEvalPartialCompound(compound, indexing.To)
	if !ok {
		return nil, false
	}
	m.primary, m.compound, m.s = primary, compound, s
	return ns[3:], true
}

func primaryInSimpleCompound(pn *parse.Primary, ev PureEvaler) (*parse.Compound, string) {
	indexing, ok := parent(pn).(*parse.Indexing)
	if !ok {
		return nil, ""
	}
	compound, ok := parent(indexing).(*parse.Compound)
	if !ok {
		return nil, ""
	}
	head, ok := ev.PurelyEvalPartialCompound(compound, indexing.To)
	if !ok {
		return nil, ""
	}
	return compound, head
}

func purelyEvalForm(form *parse.Form, seed string, upto int, ev PureEvaler) []string {
	// Find out head of the form and preceding arguments.
	// If form.Head is not a simple compound, head will be "", just what we want.
	head, _ := ev.PurelyEvalPartialCompound(form.Head, -1)
	words := []string{head}
	for _, compound := range form.Args {
		if compound.Range().From >= upto {
			break
		}
		if arg, ok := ev.PurelyEvalCompound(compound); ok {
			// TODO(xiaq): Arguments that are not simple compounds are simply ignored.
			words = append(words, arg)
		}
	}

	words = append(words, seed)
	return words
}
