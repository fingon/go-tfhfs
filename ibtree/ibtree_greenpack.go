/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:11:46 2017 mstenber
 * Last modified: Mon Dec 25 02:36:48 2017 mstenber
 * Edit time:     9 min
 *
 */

package ibtree

type Key string

type NodeDataChild struct {
	Key       Key
	Value     string
	childNode *Node // .. if any loaded ..
}

type NodeData struct {
	// Leafy indicates if Node is BTree leaf.
	// If so, values are whatever the client is storing there.
	// If not, values are block ids of child nodes.
	Leafy bool

	Children []*NodeDataChild
}
