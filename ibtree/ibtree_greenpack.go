/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:11:46 2017 mstenber
 * Last modified: Thu Feb  7 09:47:31 2019 mstenber
 * Edit time:     10 min
 *
 */

package ibtree

type Key string

type NodeDataChild struct {
	Key       Key    `zid:"0"`
	Value     string `zid:"1"`
	childNode *Node  `zid:"2"` // .. if any loaded ..
}

type NodeData struct {
	Leafy    bool             `zid:"0"`
	Children []*NodeDataChild `zid:"1"`
	// Leafy indicates if Node is BTree leaf.
	// If so, values are whatever the client is storing there.
	// If not, values are block ids of child nodes.
}
