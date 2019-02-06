/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2019 Markus Stenberg
 *
 * Created:       Wed Feb  6 11:55:05 2019 mstenber
 * Last modified: Wed Feb  6 11:57:16 2019 mstenber
 * Edit time:     2 min
 *
 */

package pb

func StringToBlockId(s string) *BlockId {
	return &BlockId{Id: []byte(s)}
}
