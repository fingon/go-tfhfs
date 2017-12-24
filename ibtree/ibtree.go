/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Mon Dec 25 01:26:29 2017 mstenber
 * Edit time:     2 min
 *
 */

// ibtree package provides a functional b-tree that consists of nodes,
// with N children each, that are either leaves or other nodes.
//
// It has built-in persistence, and Merkle tree-style hash tree
// behavior; root is defined by simply root node's hash, and similarly
// also all the children.
package ibtree

type IBNode struct {
}

type IBTree struct {
}
