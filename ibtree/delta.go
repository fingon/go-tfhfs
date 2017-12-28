/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:53:36 2017 mstenber
 * Last modified: Thu Dec 28 19:09:15 2017 mstenber
 * Edit time:     51 min
 *
 */

package ibtree

type IBDeltaCallback func(old, new *IBNodeDataChild)

// IterateDelta produces callback for every difference in the leaves
// local tree as opposed to 'other'.
//
// Some clever things are done to avoid pointless subtree iteration.
// Still, this is pretty expensive operation and should be done only
// in background.
func (self *IBNode) IterateDelta(original *IBNode, deltacb IBDeltaCallback) {
	var st, st0 IBStack
	st0.nodes[0] = original
	st.nodes[0] = self

	for {
		c0 := st0.child()
		if c0 == nil {
			if st0.top > 0 {
				st0.popNode()
				st0.nextIndex()
				continue
			}
		}
		c := st.child()
		if c == nil {
			if st.top > 0 {
				st.popNode()
				st.nextIndex()
				continue
			}
		}

		if c == nil && c0 == nil {
			return
		}
		//log.Printf("@delta c0:%v@%v c:%v@%v",
		//c0, st0.indexes[:st0.top+1],
		//	c, st.indexes[:st.top+1])

		n := st.node()
		n0 := st0.node()

		// Best cast first - they seem to be same exactly;
		// direct omit and no need to recurse
		if n.Leafy == n0.Leafy && c != nil && c0 != nil && *c == *c0 {
			st0.nextIndex()
			st.nextIndex()
			continue
		}

		// Look harder at the one with lower key
		if c == nil || c0 == nil || c.Key != c0.Key {
			cst := &st
			if c == nil || (c0 != nil && c.Key > c0.Key) {
				cst = &st0
			}

			// cst has the lower key
			if !cst.node().Leafy {
				// Go deeper
				cst.pushCurrentIndex()
				continue
			}

			if cst == &st0 {
				deltacb(cst.child(), nil)
			} else {
				deltacb(nil, cst.child())
			}
			cst.nextIndex()
			continue
		}

		// Keys are same. If not leaves, go deeper.
		push0 := !n0.Leafy
		push := !n.Leafy
		if push0 || push {
			if push0 {
				st0.pushCurrentIndex()
			}
			if push {
				st.pushCurrentIndex()
			}
			continue
		}

		// Both keys same and they're in leafy
		// nodes. Hooray. It's update.
		deltacb(c0, c)

		st0.nextIndex()
		st.nextIndex()
	}
}
