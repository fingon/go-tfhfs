/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:11:46 2017 mstenber
 * Last modified: Mon Dec 25 01:26:52 2017 mstenber
 * Edit time:     7 min
 *
 */

package ibtree

type IBNodeDataChild struct {
	Key       []string
	Value     []string
	childNode *IBNode // .. if any loaded ..
}

type IBNodeData struct {
	// Leafy indicates if Node is BTree leaf.
	// If so, values are whatever the client is storing there.
	// If not, values are block ids of child nodes.
	Leafy    bool
	Children []*IBNodeDataChild
}
