// SPDX-License-Identifier: Apache-2.0

package assembly

import (
	"fmt"
	"strconv"
	"strings"
)

// Node is an S-expression as KiCad writes it: a list, or an atom that
// was quoted or not.
type Node struct {
	Atom   string
	Quoted bool
	List   []*Node
	IsList bool
}

// Head is a list's first atom: "symbol" for (symbol "R" ...).
func (n *Node) Head() string {
	if !n.IsList || len(n.List) == 0 || n.List[0].IsList {
		return ""
	}
	return n.List[0].Atom
}

// Arg is the i-th atom after the head, or "".
func (n *Node) Arg(i int) string {
	k := 0
	for _, c := range n.List[1:] {
		if !c.IsList {
			if k == i {
				return c.Atom
			}
			k++
		}
	}
	return ""
}

// Children are the sub-lists with this head.
func (n *Node) Children(head string) []*Node {
	var out []*Node
	for _, c := range n.List {
		if c.Head() == head {
			out = append(out, c)
		}
	}
	return out
}

// Child is the first sub-list with this head, or nil.
func (n *Node) Child(head string) *Node {
	for _, c := range n.List {
		if c.Head() == head {
			return c
		}
	}
	return nil
}

// Copy is a deep copy.
func (n *Node) Copy() *Node {
	c := *n
	c.List = nil
	for _, k := range n.List {
		c.List = append(c.List, k.Copy())
	}
	return &c
}

// Parse reads one S-expression.
func Parse(src string) (*Node, error) {
	p := &parser{s: src}
	p.space()
	n, err := p.node()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, fmt.Errorf("offset %d: text after the expression", p.i)
	}
	return n, nil
}

type parser struct {
	s string
	i int
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

func (p *parser) space() {
	for p.i < len(p.s) && isSpace(p.s[p.i]) {
		p.i++
	}
}

func (p *parser) node() (*Node, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("offset %d: unexpected end", p.i)
	}
	switch p.s[p.i] {
	case '(':
		p.i++
		n := &Node{IsList: true}
		for {
			p.space()
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("offset %d: unclosed list", p.i)
			}
			if p.s[p.i] == ')' {
				p.i++
				return n, nil
			}
			c, err := p.node()
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, c)
		}
	case ')':
		return nil, fmt.Errorf("offset %d: unexpected )", p.i)
	case '"':
		p.i++
		var b strings.Builder
		for p.i < len(p.s) && p.s[p.i] != '"' {
			c := p.s[p.i]
			if c == '\\' && p.i+1 < len(p.s) {
				p.i++
				switch c = p.s[p.i]; c {
				case 'n':
					c = '\n'
				case 't':
					c = '\t'
				}
			}
			b.WriteByte(c)
			p.i++
		}
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("offset %d: unclosed string", p.i)
		}
		p.i++
		return &Node{Atom: b.String(), Quoted: true}, nil
	default:
		j := p.i
		for p.i < len(p.s) && !isSpace(p.s[p.i]) && p.s[p.i] != '(' && p.s[p.i] != ')' && p.s[p.i] != '"' {
			p.i++
		}
		return &Node{Atom: p.s[j:p.i]}, nil
	}
}

// Write writes n as KiCad does: one list per line, nested lists indented
// by a tab, atoms on their list's line.
func Write(b *strings.Builder, n *Node, indent int) {
	if !n.IsList {
		b.WriteString(atom(n))
		return
	}
	b.WriteString("(")
	nested := false
	for i, c := range n.List {
		if c.IsList {
			b.WriteString("\n")
			b.WriteString(strings.Repeat("\t", indent+1))
			Write(b, c, indent+1)
			nested = true
			continue
		}
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(atom(c))
	}
	if nested {
		b.WriteString("\n")
		b.WriteString(strings.Repeat("\t", indent))
	}
	b.WriteString(")")
}

func atom(n *Node) string {
	if n.Quoted {
		return strconv.Quote(n.Atom)
	}
	return n.Atom
}

// A, Q and L build nodes: an atom, a quoted atom, a list.
func A(s string) *Node { return &Node{Atom: s} }
func Q(s string) *Node { return &Node{Atom: s, Quoted: true} }
func L(head string, k ...*Node) *Node {
	return &Node{IsList: true, List: append([]*Node{A(head)}, k...)}
}
