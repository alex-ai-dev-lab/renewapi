package controller

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseLogTypeFilter(t *testing.T) {
	values, err := parseLogTypeFilter(" 2,5,2 ")
	require.NoError(t, err)
	require.Equal(t, []int{2, 5}, values)
	values, err = parseLogTypeFilter("")
	require.NoError(t, err)
	require.Nil(t, values)
	for _, value := range []string{"2,", "-1", "8", "2 OR 1=1", "0,1,2,3,4,5,6,7,0"} {
		_, err = parseLogTypeFilter(value)
		require.Error(t, err, value)
	}
}
