package common

import (
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPageQueryBoundsAndAliases(t *testing.T) {
	for _, tc := range []struct {
		query      string
		page, size int
	}{
		{"p=-2&size=-1", 1, 100}, {"p=0&size=0", 1, 100},
		{"p=2&ps=20", 2, 20}, {"p=3&page_size=10&size=50", 3, 10},
		{"p=1.5&size=2.5", 1, 100}, {"p=1&size=101", 1, 100},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
			p := GetPageQuery(c)
			require.Equal(t, tc.page, p.Page)
			require.Equal(t, tc.size, p.PageSize)
		})
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?p=9223372036854775807&size=100", nil)
	p := GetPageQuery(c)
	require.GreaterOrEqual(t, p.GetStartIdx(), 0)
	require.LessOrEqual(t, p.GetEndIdx(), math.MaxInt)
}
