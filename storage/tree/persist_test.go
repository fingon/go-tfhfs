/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Mar 13 13:29:24 2018 mstenber
 * Last modified: Tue Mar 13 13:35:56 2018 mstenber
 * Edit time:     3 min
 *
 */

package tree

import (
	"testing"

	"github.com/stvp/assert"
)

func TestInMemoryPersist(t *testing.T) {
	t.Parallel()
	p := inMemoryFile{}
	ls := LocationSlice{LocationEntry{Offset: 1, Size: 3}}
	td := []byte("foo")
	p.WriteData(ls, td)
	td2 := p.ReadData(ls)
	assert.Equal(t, td, td2)
	ls2 := LocationSlice{LocationEntry{Offset: 2, Size: 3}}
	p.WriteData(ls2, td)
	ls3 := LocationSlice{LocationEntry{Offset: 0, Size: 5}}
	b := p.ReadData(ls3)
	assert.Equal(t, b, []byte{0, 102, 102, 111, 111}) // 0 + ffoo
}
