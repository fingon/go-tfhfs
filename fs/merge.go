/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 10:43:03 2018 mstenber
 * Last modified: Wed Jan 17 13:11:16 2018 mstenber
 * Edit time:     50 min
 *
 */

package fs

import (
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
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
// handed in as the Transaction.  If local is set, all changes are
// assumed to be dealt with on per-leaf node difference
// basis. Otherwise metadata of the particular inode is used to
// determine which version of the truth is preferrable.
func MergeTo3(tr *hugger.Transaction, src, dst *ibtree.Node, local bool) {
	t := tr.IB()
	m := make(map[uint64]mergeVerdict)
	isdir := make(map[uint64]bool)
	isdir[1] = true
	dst.IterateDelta(src,
		func(oldC, newC *ibtree.NodeDataChild) {
			var k BlockKey
			var c *ibtree.NodeDataChild
			// My god, if I only had ternary operator..
			if oldC == nil {
				c = newC
			} else {
				c = oldC
			}
			k = BlockKey(c.Key)

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
							if dstMeta.IsDir() {
								isdir[k.Ino()] = true
							}
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
			if v == MV_NONE {
				return
			}

			// In non-local mode, only MV_NEW changes are
			// worth propagating unless it is a directory,
			// which we will handle anyway as if it was
			// local mode. (Files are separate concerns
			// from each other and can be modified
			// atomically.)
			if !local && v != MV_NEW && !isdir[k.Ino()] {
				return
			}
			if newC == nil {
				// Delete
				cv := t.Get(oldC.Key)
				if cv != nil && (*cv == oldC.Value || v == MV_NEW) {
					mlog.Printf2("fs/merge", " delete %x", oldC.Key)
					t.Delete(oldC.Key)
				}
			} else if oldC == nil {
				// Insert
				cv := t.Get(newC.Key)
				if cv == nil || v == MV_NEW {
					mlog.Printf2("fs/merge", " insert %x", newC.Key)
					t.Set(newC.Key, newC.Value)
				}
			} else {
				// Update
				cv := t.Get(oldC.Key)
				if (cv != nil && *cv == oldC.Value) || v == MV_NEW {
					mlog.Printf2("fs/merge", " update %x", newC.Key)
					t.Set(newC.Key, newC.Value)
				}
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
