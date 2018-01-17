/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 10:43:03 2018 mstenber
 * Last modified: Wed Jan 17 11:38:06 2018 mstenber
 * Edit time:     34 min
 *
 */

package fs

import (
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
)

type mergeVerdict int

const (
	// MV_NEW indicates the preference in src->dst tree delta
	MV_NEW mergeVerdict = iota

	// MV_EXISTING indicates the preference of status quo
	MV_EXISTING

	// MV_NONE indicates preference for getting rid of ALL data
	// for particular inode.
	MV_NONE
)

// MergeTo3 performs 3-way merge. It iterates changes in orig -> new,
// and then compares them with the current state in the tree that is
// handed in as the IBTransaction.
func MergeTo3(t *ibtree.IBTransaction, src, dst *ibtree.IBNode, local bool) {
	m := make(map[uint64]mergeVerdict)
	dst.IterateDelta(src,
		func(oldC, newC *ibtree.IBNodeDataChild) {
			var k BlockKey
			if oldC == nil {
				k = BlockKey(newC.Key)
			} else {
				k = BlockKey(oldC.Key)
			}

			ino := k.Ino()
			v, ok := m[ino]
			if !ok {
				// Peculiar, this should not happen;
				// ignore the changes that do not
				// update metadata
				if k.SubType() == BST_META {

					op := t.Get(k.IB())
					if newC == nil {
						if op == nil {
							// both deleted; does not matter
							v = MV_NONE
						} else {
							srcMeta := decodeInodeMeta(oldC.Value)
							otherMeta := decodeInodeMeta(*op)
							if srcMeta.StCtimeNs >= otherMeta.StCtimeNs {
								v = MV_NONE
							} else {
								v = MV_EXISTING
							}
						}
					} else {
						if op == nil {
							// It changed on our
							// side too -> prefer
							// ours
							v = MV_NEW
						} else {
							dstMeta := decodeInodeMeta(newC.Value)
							otherMeta := decodeInodeMeta(*op)
							if dstMeta.StCtimeNs > otherMeta.StCtimeNs {
								v = MV_NEW
							} else {
								v = MV_EXISTING
							}
						}
					}
					if !local {
						m[ino] = v

					}
				} else if !local {
					// Non-local mode, but no
					// metadata change is bogus
					return
				} else {
					// local handling; prefer new
					// over old (This should occur
					// only in short-lived stuff
					// like e.g. writes to
					// non-conflicting blocks)
					v = MV_NEW
				}
			}

			// MV_NONE is handled separately as
			// DeleteRange (more efficient and also more
			// correct).
			if v != MV_NEW {
				return
			}
			if newC == nil {
				// Delete
				v := t.Get(oldC.Key)
				if v != nil {
					mlog.Printf2("fs/fstransaction", " delete %x", oldC.Key)
					t.Delete(oldC.Key)
				}
			} else if oldC == nil {
				// Insert
				mlog.Printf2("fs/fstransaction", " insert %x", newC.Key)
				t.Set(newC.Key, newC.Value)
			} else {
				// Update
				mlog.Printf2("fs/fstransaction", " update %x", newC.Key)
				t.Set(newC.Key, newC.Value)
			}
		})
	for ino, v := range m {
		if v == MV_NONE {
			// TBD: Should we iterate through children to
			// keep nlinks up to date?
			//IterateInoSubTypeKeys(t, ino, fs.DIR_NAME2INODE,
			// func(key BlockKey) bool {
			// ..
			// })

			k1 := NewBlockKey(ino, BST_NONE, "").IB()
			k2 := NewBlockKey(ino, BST_LAST, "").IB()
			t.DeleteRange(k1, k2)
		}
	}
}
