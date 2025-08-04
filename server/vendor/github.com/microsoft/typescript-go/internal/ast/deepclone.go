package ast

import "github.com/microsoft/typescript-go/internal/core"

// Ideally, this would get cached on the node factory so there's only ever one set of closures made per factory
func getDeepCloneVisitor(f *NodeFactory, syntheticLocation bool) *NodeVisitor {
	var visitor *NodeVisitor
	visitor = NewNodeVisitor(
		func(node *Node) *Node {
			visited := visitor.VisitEachChild(node)
			if visited != node {
				return visited
			}
			c := node.Clone(f) // forcibly clone leaf nodes, which will then cascade new nodes/arrays upwards via `update` calls
			// In strada, `factory.cloneNode` was dynamic and did _not_ clone positions for any "special cases", meanwhile
			// Node.Clone in corsa reliably uses `Update` calls for all nodes and so copies locations by default.
			// Deep clones are done to copy a node across files, so here, we explicitly make the location range synthetic on all cloned nodes
			if syntheticLocation {
				c.Loc = core.NewTextRange(-1, -1)
			}
			return c
		},
		f,
		NodeVisitorHooks{
			VisitNodes: func(nodes *NodeList, v *NodeVisitor) *NodeList {
				if nodes == nil {
					return nil
				}
				// force update empty lists
				if len(nodes.Nodes) == 0 {
					return nodes.Clone(v.Factory)
				}
				return v.VisitNodes(nodes)
			},
			VisitModifiers: func(nodes *ModifierList, v *NodeVisitor) *ModifierList {
				if nodes == nil {
					return nil
				}
				// force update empty lists
				if len(nodes.Nodes) == 0 {
					return nodes.Clone(v.Factory)
				}
				return v.VisitModifiers(nodes)
			},
		},
	)
	return visitor
}

func (f *NodeFactory) DeepCloneNode(node *Node) *Node {
	return getDeepCloneVisitor(f, true /*syntheticLocation*/).VisitNode(node)
}

func (f *NodeFactory) DeepCloneReparse(node *Node) *Node {
	if node != nil {
		node = getDeepCloneVisitor(f, false /*syntheticLocation*/).VisitNode(node)
		SetParentInChildren(node)
		node.Flags |= NodeFlagsReparsed
	}
	return node
}

func (f *NodeFactory) DeepCloneReparseModifiers(modifiers *ModifierList) *ModifierList {
	return getDeepCloneVisitor(f, false /*syntheticLocation*/).VisitModifiers(modifiers)
}
