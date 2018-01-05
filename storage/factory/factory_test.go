/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:28:57 2018 mstenber
 * Last modified: Fri Jan  5 16:29:17 2018 mstenber
 * Edit time:     1 min
 *
 */

package factory

import (
	"testing"

	"github.com/stvp/assert"
)

func TestList(t *testing.T) {
	t.Parallel()
	assert.Equal(t, len(List()), len(backendFactories))
}
